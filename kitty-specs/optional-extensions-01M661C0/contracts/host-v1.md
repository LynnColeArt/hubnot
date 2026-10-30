# Proposed host contract — hn.extension/1

Planning contract, not an existing command/API. One host-owned executable/argv process
has a private stdio session bound to one local registration and canonical repository.

## Framing and envelopes

A frame is a4-byte unsigned big-endian byte length followed by one strict UTF-8 JSON
object, maximum8MiB. Reject zero/over-limit lengths before allocating that payload,
duplicate keys, malformed Unicode, nonobjects and unknown envelope fields.
Do not infer session identity from incoming module/repository strings.

Request shape:
~~~json
{"protocol":"hn.extension/1","request_id":"r1","method":"capabilities.list","params":{}}
~~~
Response shape:
~~~json
{"protocol":"hn.extension/1","request_id":"r1","ok":true,"data":{"capabilities":[]},"error":null}
~~~
Error shape:
~~~json
{"protocol":"hn.extension/1","request_id":"r2","ok":false,"data":null,"error":{"code":"denied_access","message":"action is not granted","recovery":"do_not_retry"}}
~~~
Request IDs are printable non-whitespace ASCII1..128bytes and correlate one session
request only; they are not durable mutation identities. State-mutating methods carry
full actual actor and actor-scoped operation key, immutable typed input and exact
expected heads. All responses echo protocol/request ID; session metadata exposes
the host-selected extension/repository binding as public diagnostics.

## First method vocabulary

- capabilities.list: declared/granted/enabled/compatibility/availability and recovery class.
- identity.public: noninitializing public inspection.
- resource.read, resource.mutate, resource.operation: supported native family/action.
- artifact.read, artifact.mutate, artifact.operation, artifact.search: typed generic records.
- artifact.type.read, artifact.type.publish: exact portable schema declarations.
- changes.capture, changes.page, changes.since: bounded accepted visible observations.

Separate grants: read, observe, create, revise, resolve, type.publish and raw.inspect,
scoped to canonical repository and native family or custom type namespace.
A generic write grant is not a resolve, policy or execution grant. Params are typed
allowlisted objects, never caller-supplied command argv. Validate grants before native
mutation/preflight creates a private intent. Recheck actual active actor on every write.
Missing signer does not cause initialization; actor mismatch refuses.

Native families: issue, project, project-item, discussion, discussion-reply and workflow
definitions. Use existing exact request, expected-head, conflict and operation semantics.
Memory mutations, proposal/review/decide/merge, policy mutation, identity lifecycle,
run/runner/audit/automerge execution and implicit sync/migrate apply are unsupported.
Their inventory status is not inferred from installation or a version string.

## Diagnostics and recovery

Closed host codes: denied_access, incompatible_version, unavailable_module,
unsupported_capability, resource_limit, cancelled, uncertain_operation,
checkpoint_invalidated, verification_failure, invalid_input, repository_error.
Native stale_revision, concurrent_update, permission_denied, admission_refused,
admission_pending and operation replay/conflict codes retain their actual native
identity/context; do not collapse them into success or universal retryable.
Before shipping, enumerate actual native codes from the integrated beta source and
add verbatim examples without renaming them. This is a deliberate native-code seam,
not permission for unknown host errors to pass silently.

Recovery classifications: do_not_retry, retry_identical, inspect_operation,
rescan_required. Client persists intent before submit, uses original actor/key on
lookup/replay, and never regenerates mutation content from the latest state.
An interrupted dispatched write remains uncertain unless a definitive result is proven.
Recovery identity includes the native/custom protocol as well as actual actor/key;
existing native cross-family operation-key scope stays unchanged. No cross-protocol
global exactly-once promise is introduced.

## Lifecycle and observation bounds

Startup10s; reads default30s and mutations default240s, maximum240s capped by
stricter native context; stderr64KiB;
one outstanding request/session and four sessions/canonical repository.
Explicit cancellation/EOF/disable terminates, then kills after2s, verifies exit by5s.
These are host watchdogs, not a hard bound on all kernel or legacy synchronous work.
No source-controlled module code runs during ordinary reads or manifest inspection.

Page limit1..200/default20 plus8MiB frame; captured set10000entries/16MiB, expires10minutes.
If full state cannot fit, return resource_limit/smaller-page guidance, never truncate it.
Changes bind actual repo/view/source cuts and selected authority/policy/moderation.
Return checkpoint_invalidated/rescan_required if the old cut cannot be validated.
Duplicates use full event identity plus repository/view. Full-ID ordering is presentation;
it is neither causal order nor an exactly-once promise. Invalid history is a failure,
not empty changes. Observation causes no explicit transport sync or workflow execution.
