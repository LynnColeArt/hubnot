# Optional local extensions

Hubnot's native commands still need only the supplied `hn` executable and Git.
Extensions add an explicitly started local executable; there is no automatic
installation, discovery, provider connection, account, daemon, or required language
runtime. Generic [artifact commands](artifacts-v1.md) work without extensions.
The [consumer guide](extension-consumers.md) contains optional source examples.

The current host protocol is **`hn.extension/1`**, carried over private stdio pipes.
Manifest, registry, repository declaration and enablement versions are respectively
`hn.extension-manifest/1`, `hn.extension-registry/1`,
`hn.extension-declarations/1`, and `hn.extension-enablement/1`.
These describe the implementation in this source tree; they do not assert that an
older released executable supports the new protocols.

## Explicit lifecycle

A manifest declares interest, not permission or an executable path:

```json
{"protocol":"hn.extension-manifest/1","module_id":"example.inventory/publisher","module_version":"1","host_protocol":"hn.extension/1","capabilities":["identity.public","artifact.type.publish","artifact.mutate"]}
```

With an explicitly supplied, trusted executable and manifest:

```sh
hn extension register --manifest /absolute/private/manifest.json \
  --executable /absolute/private/inventory-model \
  --arg --input --arg /absolute/private/command.json \
  --arg --output --arg /absolute/private/result.json --json
hn extension inspect example.inventory/publisher --json
hn extension list --json
hn extension enable example.inventory/publisher \
  --grant native:identity=read \
  --grant artifact:example.inventory/*=create,revise,type.publish --json
hn extension run example.inventory/publisher --json
hn extension disable example.inventory/publisher --json
hn extension remove example.inventory/publisher --json
```

These are lifecycle commands, not a complete publication recipe: prepare the
sample's command file using [its guide](../examples/extensions/inventory-model/README.md)
before `run`. `--arg` repeats to supply each literal argument; no shell interprets
it. Register can read its manifest from `--manifest -`. Inspection, registration,
enablement, disable and removal never launch or probe a module. Only `run` launches.

Registration records the physically resolved absolute executable, its byte digest,
argv, manifest digest and a fresh private registration instance. Exact duplicate
registration is idempotent; changed registration requires explicit removal and
registration. Rehashing/rebinding occurs before launch. A changed/missing executable
must be inspected and repaired explicitly, not automatically replaced or trusted.
Remove/re-register creates a new instance, so every repository must explicitly
enable it again; stale enablement elsewhere cannot authorize the new instance.

Enablement binds the physical Git common directory, registration and a new epoch.
Linked worktrees share this local repository binding; a clone does not inherit it.
Each `enable` supplies the **complete** desired grant set. Enable/disable/grant edits
revoke older sessions. Disable/remove stop only owned processes and verify cleanup;
removal preserves signed public records. A failed cleanup is a failure, not a
successful disable. There is no automatic repository scan to restore old grants.

`.hn/extensions.json` is an inert repository declaration of module IDs/versions.
Cloning or reading it never installs, selects an executable, grants, initializes
identity or activates doctrine. Ordinary native commands do not parse registration,
start/probe extensions or write extension state, even when an extension is broken.

## Capabilities and authority

Do not collapse these independent facts:

| Field | Meaning |
|---|---|
| `declared` | The manifest names the method. It grants nothing. |
| `granted` | At least one applicable selector/action exists; each request still needs its exact scope. |
| `enabled` | The registration is explicitly enabled at the current repository epoch. |
| `compatible` | The exact host protocol is compatible; versions are not implicit ranges. |
| `availability` | `not_probed` for inert compatible inspection, `unavailable` for missing/changed/incompatible input, `observed` after a successful session handshake. |

`inspect` cannot establish observed availability. A supported method can be denied
or its module unavailable. A permanently unsupported surface cannot be enabled by
installation, a broad grant, a namespace, a signature or a schema.

Selectors are `native:FAMILY`, `native:identity`, exact
`artifact:NAMESPACE/NAME`, or `artifact:NAMESPACE/*`. The sole special selector
`artifact:opaque` permits only `raw.inspect`. There is no path expansion or general
glob. Actions are exactly `read`, `observe`, `create`, `revise`, `resolve`,
`type.publish`, `raw.inspect`. Native identity permits only read; type publication
and raw inspection are custom-only. Grants mediate host calls, not OS access.

## Complete first-host method matrix

All fourteen methods are listed once by `capabilities.list`, including undeclared
or ungranted rows. The closed parameter/result shapes are specified in the
[frozen host contract](../kitty-specs/optional-extensions-01M661C0/contracts/host-v1.md#4-strict-method-types-and-grants).

| Method | Required selector/action | Behavior |
|---|---|---|
| `capabilities.list` | Enabled compatible session | Session binding, full inventory and unsupported surfaces. |
| `identity.public` | `native:identity=read` | Public initialized flag/actor/name/key; no initialization. |
| `resource.read` | `native:FAMILY=read` | Accepted visible native view; full ID or explicit list, limit/cursor. |
| `resource.mutate` | `native:FAMILY=create`, `revise` or `resolve` | Full state, exact expected parents, actual actor and durable operation key. Open and issue comment use create. |
| `resource.operation` | `native:FAMILY=read` | Exact native actor/key lookup; wrong family is operation_conflict. |
| `artifact.read` | Matching type=read; opaque alone=`artifact:opaque=raw.inspect` | view/heads/history/inspect; opaque is explicit public quarantine only. |
| `artifact.mutate` | Matching type=create/revise/resolve | Open=create; revise/convert=revise; resolve=resolve. |
| `artifact.operation` | Matching type=read | Exact custom actor/key lookup, including type publication; wrong type conflicts. |
| `artifact.search` | Read on every selected custom type | Literal or regex and optional type/shape, bounded complete selection. |
| `artifact.type.read` | Matching type=read | Exact signed declaration or filtered listing. |
| `artifact.type.publish` | Payload type=type.publish | Explicit signed declaration; no namespace ownership or activation. |
| `changes.capture` | **Read and observe** on every selected family/type | Explicit nonempty selection, bounded replacement snapshot. |
| `changes.page` | Same read/observe, selection and epoch | Continue a still-valid session capture. |
| `changes.since` | Same read/observe and valid checkpoint | Begin a complete current replacement, not a sparse diff. |

The six native families are `issue`, `project`, `project-item`, `discussion`,
`discussion-reply`, `workflow`. All support open/revise/resolve; issue additionally
supports comment. There is no native convert or close/reopen convenience method;
status changes are explicit full revisions. Workflow definitions are stored data,
not execution. Native nested state retains [issue](issues-v1.md),
[project/item](projects-v1.md), [discussion/reply](discussions-v1.md) and
[workflow](workflows-v1.md) validation and limits.

Native adapters retain normal selected authority, contribution admission,
moderation, retention and visibility. Signature validity is not admission or
visibility. Admission is `null`, `admitted`, `pending` or `hidden`. Custom mutations
and custom change captures under selected contribution policy are unsupported and
refuse before signing/intents/publication or capture state. Native governed CRUD
is supported through its existing checks; generic custom data is not a governed feed.

The explicit unsupported inventory is `memory.mutate`, `proposal`, `review`,
`decide`, `merge`, `policy.mutate`, `identity.mutate`, `run`, `runner`, `audit`,
`automerge`, `sync.apply`, `migration.apply`. Recovery is `do_not_retry`.
There is also no host memory recall/grep/read API, provider API, arbitrary command
bridge, key initialization/rotation/lifecycle, transport apply, policy evaluation
or executable schema validator. The management command `extension run` starts a
module; the business method `run` is unsupported. Use separately authorized native
commands for native governance/execution; extension grants confer no such authority.

## Framing and finite sessions

A frame is a four-byte unsigned big-endian length followed by one UTF-8 JSON object,
payload 1..8388608 bytes. The host sends its bound hello, then requires exactly
`{"protocol":"hn.extension/1","ready":true}` before business requests. Requests
contain protocol, unique session request_id, known method, closed params, optionally
timeout_ms. Correlation IDs are not durable mutation keys.

Responses contain exactly protocol, request_id, ok, data, error. Success has
ok=true/error=null; failure has ok=false/data=null. Consume the **entire** response,
validate its exact request ID and method-specific closed shape, and handle it
before another request. Only one business request is outstanding. The host's
bounded final-byte observation allowance does not permit client pipelining, nor
prove physical send/read order or durable peer receipt.

A cancellation control is exactly `{"protocol":"hn.extension/1","cancel":"ID"}`
for the currently outstanding request; it has no separate acknowledgement. Invalid
control/premature business frames close the session under the same settlement and
uncertainty rules. Unknown/duplicate fields, duplicate keys, invalid Unicode,
non-safe/noninteger numbers, trailing data and wrong types refuse.

| Bound | Value |
|---|---|
| Startup, including validation/hello/ready | 10 seconds |
| Default read / mutation request | 30 / 240 seconds |
| Optional timeout_ms | 1..240000; cannot extend a stricter native deadline |
| Page | 1..200; encoded response still <=8 MiB |
| Session stderr | 65536 bytes; byte 65537 fails resource_limit |
| Sessions per physical repository | 4 |
| Terminate-to-kill / observed child exit | 2 / 5 seconds |
| Manifest/declaration / registry/enablement | 128 KiB / 1 MiB |
| Grants / registrations | 256 / 128 |

Absolute deadlines include partial frames, encoding and blocked response writes.
Revocation is checked before each request and at most every 250 ms. Idle hosts hold
no contribution write lock. Cancellation joins writers; synchronous kernel I/O can
outlast watchdog budgets, so these are not a hard kernel-I/O SLA. See the
[complete limit table](../kitty-specs/optional-extensions-01M661C0/contracts/host-v1.md#7-cli-private-bounds-and-required-conformance).

For inert wire inspection only:

```sh
hn extension frame inspect --input /absolute/request.frame --kind request --json
hn extension frame inspect --input /absolute/success.frame --kind response \
  --expected-request-id r1 --method identity.public --json
```

Supply exactly one raw length-prefixed frame through EOF, not unframed JSON or
Base64. hello/ready/request forbid correlation flags. Cancel requires matching
expected-request-id and forbids method. Correlated success requires matching ID
and method; failure requires matching ID and forbids method. Terminal failure
with null ID forbids both. `--timeout-ms` here is 1..30000 (default 30000).
Inspection checks typed values, not authorization, live session ordering, schema,
signature integrity or dispatch.

## Closed errors and immutable recovery

Error is exactly `{origin,code,message,recovery,operation}`. Message is sanitized
UTF-8 <=4096 bytes. Operation is null or original `{protocol,actor,operation}`,
where protocol is `hn/0` or `hn-artifact/1`.

| Origin | Closed code inventory |
|---|---|
| host | denied_access, incompatible_version, unavailable_module, unsupported_capability, resource_limit, cancelled, uncertain_operation, checkpoint_invalidated, verification_failure, invalid_input, not_found, repository_error |
| native | actor_mismatch, admission_pending, admission_refused, ambiguous_id, concurrent_update, invalid_cursor, invalid_history, invalid_input, issue_conflict, not_found, operation_conflict, permission_denied, repository_error, resource_conflict, resource_limit, stale_cursor, stale_revision |
| artifact | invalid_input, resource_limit, verification_failure, invalid_history, incomplete_dependency, unsupported_schema, actor_stream_conflict, operation_conflict, stale_revision, resource_conflict, concurrent_update, not_found, repository_error, cancelled, uncertain_operation |

Recovery is guidance, never automatic retry:

| Recovery | Required consumer action |
|---|---|
| `do_not_retry` | Stop this intent. Inspect stale/conflict/refusal; a changed request needs a new explicit key. |
| `retry_identical` | Inspect/settle first, then only the original actor/key/content/parents; native concurrent_update/repository_error and artifact concurrent_update use this. |
| `inspect_operation` | Query the original operation before new work; pending admission or dispatched unknown outcome is not absence. |
| `rescan_required` | Discard incomplete staging, retain old committed set/checkpoint, explicitly restart capture. |

Grant/unsupported/input/signature/changed-replay/stale/conflict/admission refusal
normally use do_not_retry. Native invalid_cursor/stale_cursor and host invalidated
checkpoint use rescan_required. Before-dispatch cancel is cancelled/do_not_retry
with no signing, intent or public effect. After dispatch, lost acknowledgement,
crash/EOF/cancel/revocation or oversized success response can leave
uncertain_operation/inspect_operation with the **original** tuple. A completed
native refusal retains its actual code even if its reply is lost. No detached
writer may publish after a definitive refusal. A completed public success and a
completed consumer receipt are different observations.

Persist the original actual actor/public-key binding, protocol, key, kind, target,
full content and exact parents before sending a mutation. Native keys span all
native families under hn/0; custom keys span all custom types/kinds under
hn-artifact/1. They are separate scopes. Exact replay returns original identities,
even after later heads change; changed replay refuses operation_conflict. A fresh
key must not be minted merely because an acknowledgement was lost.

Operation lookup returns `found`, `verified_absent` or `uncertain`, with source_cut.
Found proves original request/result identities; current head/conflict observations
may be newer. Verified absence needs complete verified selected history **and** no
pending publication evidence. Uncertain reasons are incomplete_selection,
unavailable_history, private_intent_pending, visibility_restricted,
publication_unconfirmed. Invalid/contradictory history is an error, not absence.
The [consumer recipes](extension-consumers.md#recovery-and-observation-recipes)
show how those distinctions affect restart and refusal handling.

## Replacement observation

Select explicit sorted distinct families/exact type IDs; no implicit all. The
captured set is <=10000 entries/16 MiB canonical entries, session-bound and expires
after ten minutes. Each page rechecks binding, grants, source cut and actual
policy/authority/visible/retained view. Changes/expiry/restart/missing capture state
invalidate continuation. Captures/cursors are opaque; a completed checkpoint can
cross sessions only while its verifier and prior cut remain verifiable.

1. Keep the old committed set/checkpoint. Start empty staging for the exact selection.
2. Dedupe each page by repository/selection/resource and full current revisions.
3. Only complete=true/next_cursor=null/non-null checkpoint permits one transaction
   replacing the **whole** selected set and checkpoint, including removals.
4. On invalidation discard incomplete staging and explicitly capture afresh.

`changes.since` starts a complete replacement, even when identical. It is not a
causal event order or exactly-once write feed. Hide/restore without edits is
reconciled by completed replacement; never infer removals outside the selection.
Smaller pages can fix a page-size error, not capture capacity or an oversized
single state. Use narrower bounded selections or explicit local export. External
API write intents, receipts and reconciliation remain connector-owned.

## Process trust and private state

A module runs as the same OS user, with that user's ordinary filesystem/network
powers. This host is **not a sandbox**. Use only executables you trust; grants
mediate host requests and cannot prevent direct OS access by a malicious module.
Owned process-group cleanup is not containment of a process that escapes its group.

Registration is user-local at `os.UserConfigDir()/hubnot/extensions/registry.json`;
enablement/control/captures are under the physical common Git directory's
`hn/extensions/`. Signing keys, local custom intents, registration/argv, credentials,
connector journals, private application output and raw stderr never belong in
public artifacts or selected export. Private host directories/files use 0700/0600
and safe atomic replacement. Application journals/results are the consumer's
responsibility. See the [threat model](threat-model.md#optional-local-extension-host).
