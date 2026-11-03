# Local fake tracker reference

This separate, optional Go executable demonstrates the public
[`hn.extension/1`](../../../kitty-specs/optional-extensions-01M661C0/contracts/host-v1.md)
stdio contract. It imports one fake issue and its one comment, explicitly imports
later external versions, and exports an observed native edit. It uses only the Go
standard library. Native Hubnot commands need neither this executable nor its HTTP
service. This is a reference experiment, not a production tracker integration.

The connector owns its connection/source identity, credential, scheduling (each
explicit invocation), immutable native actor/public key/operation/content/parents,
external write intents and receipts, accepted replacement set, and checkpoint.
Hubnot signs as its actual initialized native actor. External author/time/source
metadata and comment markers are descriptive claims, including deliberately old or
future claimed times; they confer no identity, policy, execution or grant authority.
Only the exact native issue recorded in the connector's private successful import
receipt is eligible for export. Matching metadata on another issue cannot create
an association. Removing a descriptive marker from the mapped issue does not erase
the connector-owned association or grant authority to another resource.

## Build and explicitly supply the reference

From the repository root:

```sh
go build -o /absolute/private/fake-tracker ./examples/extensions/fake-tracker
```

Choose a private directory outside the repository, mode `0700`. Every JSON file
below must be mode `0600` (for example, set `umask 077` before creating it). Replace
the absolute example paths consistently. The fake service initially reads
`service.json`:

```json
{"item":{"id":"42","version":1,"title":"External issue","body":"Exact external text\n","author":"claimed external author","time":"1901-01-01T00:00:00Z","comments":[{"id":"c1","body":"External comment","author":"claimed commenter","time":"2099-01-01T00:00:00Z"}]},"receipts":{},"writes":0}
```

Put `{"token":"local-example-token"}` in private `credential.json`, then start the
service explicitly:

```sh
/absolute/private/fake-tracker serve \
  --state /absolute/private/service.json \
  --credential /absolute/private/credential.json \
  --listen 127.0.0.1:0 --ready /absolute/private/ready
```

The ready file contains the actual loopback HTTP URL. Put that URL in private
`config.json`; keep the same token, connection and source throughout recovery:

```json
{"api":"http://127.0.0.1:REPLACE_PORT","token":"local-example-token","connection":"my-local-tracker","source":"tracker:item:42","journal":"/absolute/private/journal.json","report":"/absolute/private/report.json","marker":"/absolute/private/marker.json"}
```

Explicitly create a Git repository and initialize the native identity with the
ordinary `hn init --name 'Reference author'`. No extension method initializes or
rotates identity. Supply this manifest in `manifest.json`:

```json
{"protocol":"hn.extension-manifest/1","module_id":"example.tracker/tool","module_version":"1","host_protocol":"hn.extension/1","capabilities":["identity.public","resource.read","resource.mutate","resource.operation","changes.capture","changes.page","changes.since","capabilities.list"]}
```

Register the actual executable and its fixed configuration/action-file arguments,
then grant only this repository's required actions:

```sh
hn extension register --manifest /absolute/private/manifest.json \
  --executable /absolute/private/fake-tracker \
  --arg --config --arg /absolute/private/config.json \
  --arg --plan --arg /absolute/private/plan.json --json
hn extension enable example.tracker/tool \
  --grant native:identity=read \
  --grant native:issue=read,observe,create,revise --json
```

There is no resolve grant or automatic conflict resolution. Executable replacement
requires explicit re-enablement; a changed registration, repository, signer,
connection, source, endpoint or credential cannot silently reuse this journal.
The journal pins the original public key; it contains no native signing secret.
Same-user processes retain ordinary filesystem powers; this is not a sandbox.

## Explicit actions and recovery

Write one action to private `plan.json`, then run:

```sh
hn extension run example.tracker/tool --json
```

The parent stdout is only the host's management summary. The consumer's explicit
private report contains host hello, exact requests/responses and completion or
refusal status. No credential is sent in host request parameters or public native
metadata. Consumer stderr uses fixed sanitized labels rather than URL/path/raw
HTTP errors. The report and journal are private recovery material, not public proof.

The consumer reads the full framed response, checks its exact correlation and
closed typed envelope/result, then handles it before sending another request.
Malformed JSON/types, mismatched IDs and terminal null-ID errors stop that session;
a second call cannot write another request. Ordinary correlated business refusals
remain explicit outcomes. The success validator covers this connector's eight
declared methods and native issue views; a diagnostic request cannot make an
unknown successful method or another family's shape silently acceptable. These
checks validate public values and connector bindings, while the host supplies
signature verification, accepted visibility, grants and native semantic validation.
No sleep, yield or retry compensates for a host response-boundary defect.
The supplied-binary fault test feeds complete valid responses as a positive
control, then terminal, mismatched, malformed and inconsistent responses. It
requires zero subsequent native requests, external reads or durable native
intents on those faults. This is a post-implementation control, separate from
the causal issue-acknowledgement/source-version regression recorded in the
private execution evidence.

| Plan | Meaning |
|---|---|
| `{"action":"import"}` | Persist the initial full external snapshot/version before native effects and import its issue/comment. Restart uses original saved snapshot/requests without a fresh external API read, queries original actor/operation, and exactly replays a found operation to verify its identity. Latest external content never rewrites old intent. |
| `{"action":"update"}` | Explicitly import the current external version using a distinct operation key and the saved native parent. A stale parent refuses. Any pending update settles its original operation before a fresh source read or new version/key selection, even after newer native/external edits. Its result and connector baseline are finalized together; uncertainty cannot mint a new update. |
| `{"action":"observe","fresh":true,"limit":1}` | Explicit complete replacement capture of accepted visible native issues. Stage every page without changing the old committed set. Export only the mapped issue, then atomically replace the exact selected set/checkpoint after all external reconciliation. |
| `{"action":"observe"}` | Reconcile pending external receipts first, then use the prior checkpoint for a new complete replacement. Repeated snapshots/restarts deduplicate exact revisions. |
| `{"action":"raw","request":{"method":"resource.read","params":{"family":"issue","id":null,"limit":200,"cursor":null}}}` | Diagnostic public request. No connector mutation journal is created. The host still checks the real closed shape, binding and grants. A business refusal is retained in the private report; malformed session input may instead terminate at the parent's management boundary. |

After `checkpoint_invalidated/rescan_required`, the old committed set/checkpoint
remain. The next operator decision can explicitly request `fresh:true`; captures
and cursors cannot survive session restart. No partial staging becomes a completed
view. A completed replacement removes absent entries from the connector's committed
accepted-view map. Loss of visibility does not implicitly delete the external issue.
Full IDs and timestamps are identities/descriptions, never global causal ordering.

A correlated mutation error with `inspect_operation` or `retry_identical` keeps
the original actor, operation key, parents and content pending. The private
`unsettled` field retains that response; older journals that put it in `refusal`
are recovered without discarding the response. A later explicit invocation
inspects the original operation first: `found` permits exact replay,
`verified_absent` permits only the same saved request, and `uncertain` stops.
A read-only lookup refusal also stops and leaves that write pending. Neither
case permits a fresh source read or a new key. Recovery guidance never starts
an automatic retry. A definitive mutation refusal with `do_not_retry` remains
an explicit stop for that intent.

A true concurrent native/external edit yields native `stale_revision` and/or the
fake API's `external_conflict`. The original native request and failed external
intent remain inspectable privately. Stop and choose an explicit new intent after
inspection; this reference supplies no conflict-resolution command. It does not
silently refresh parents, overwrite one side, or retry a known refusal.

The fake API accepts authenticated `GET /item`, `POST /writes`, and
`GET /writes/OPERATION_KEY`. A write supplies exactly `key`, `expected_version`,
`source`, `resource`, `revisions`, `title`, `body`, `lose_ack`. It compares the
actual current external version, persists the new state and exact receipt before
reply, and rejects changed reuse of a key. The connector persists that exact write
intent before POST. If the reply is lost, its receipt remains null and the durable
checkpoint does not advance. Restart queries this service's own receipt and compares
the full original request and resulting version before committing anything. Missing
receipt is `external_uncertain`, never proof of absence or permission to blindly
write again. This local fake service has an operation receipt API; no such facility
or globally exactly-once guarantee is claimed for arbitrary external trackers.

The optional action fields `pause` and `lose_external_ack` are fault-demonstration
controls. `pause` can be `native-sent:issue`, `native-sent:comment`,
`native-sent:update`, `page`, or `external-uncertain`. The consumer writes its real
PID/phase to the private marker and waits for `marker.json.continue` or normal host
cleanup. For native-sent pauses it has written the actual request but has not read
its acknowledgement. For `external-uncertain`, combine `lose_external_ack:true`:
the fake service really commits, then closes the HTTP socket before replying.
The process tests kill that actual consumer PID, wait for the host to fail/exit,
and start a new executable session with the same journal. Clear stale marker and
continuation files before another controlled run.

## Bounds and validation

Framing is the public 4-byte big-endian length plus JSON, payload `1..8388608`
bytes. HTTP request/response bodies use the same 8388608-byte consumption cap.
Private config/plan/credential input is at most 131072 bytes, and journal/report/
service state at most 16777216 bytes. Equality is accepted when otherwise valid;
first-over refuses `resource_limit`, preserving the preceding committed file.
An oversized private journal can prevent progress and checkpoint advancement; it
is never silently truncated. Config and plan reject unknown fields/trailing JSON
and require private regular-file permissions. Observation page limits are `1..200`
(default 20); invalid local plans stop before observation dispatch. The host retains
its own stricter request/mutation/frame/deadline and captured-set constraints.

File transactions use mode `0600`, file sync, same-directory rename and directory
sync. A nonblocking journal lock prevents simultaneous workers for the same state.
The reference binds a single issue with one initial external comment; it is not
an unbounded enterprise crawler, scheduler, transport plugin or native dependency.

The root acceptance tests launch this actual standalone binary and separate fake
HTTP service. They cover lost native issue/comment/update acknowledgement recovery,
original content/parents after source/current-head changes, observed export and
real lost HTTP response, deduplication, no echo, exact raw refusals, private export
boundaries, independent plugin-free native history transfer, partial replacement,
source/session invalidation, and real concurrent edits. Unit checks cover private
transaction/binding/lock behavior, exact byte bounds and fake API receipts/conflicts.

For exact supplied host and consumer assets, from the repository root:

```sh
HN_WP17_BINARY=/absolute/candidate/hn \
HN_WP20_CONSUMER=/absolute/candidate/fake-tracker \
  go test -count=1 -run '^TestExtensionFakeTracker' . -hn-baseline-trace /usr/bin/strace
go test -count=1 ./examples/extensions/fake-tracker
```

The same acceptance command works with the supplied combined `nogithub,noforge`
host executable. Native default tracing deliberately detects an actual forbidden
registry read as its negative control. Qualification reports bind exact source,
inputs, executable hashes and variants; these local tests make no final platform,
performance, human-adoption, beta-release or mission-completion claim.
