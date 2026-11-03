# Native runner efficiency contract

## Commands

- Existing runner once/watch invocations preserve their actor, pipeline, backend
  and remote selection. New optional --skip-stale-base requires an explicit
  --target refs/heads/NAME. Invalid or partial option combinations refuse.
- hn run withdraw FULL_REQUEST --reason TEXT explicitly signs queued withdrawal;
  output binds the original request and resulting event, and reminds operators
  that normal sync publishes it. Exact retries append nothing. Changed reasons
  or a different signer refuse without state mutation.
- hn run help and hn run capabilities --json are static keyless reads.
  The capabilities schema identifies supported withdrawal semantics and queued-only
  cancellation; it grants no trust, execution permission or extension capability.
- run list/show report withdrawal with its exact event/request identity.
- Future explicit and runner execution of an observed withdrawn request refuses
  or skips before backend work. Historical results remain verifiable.

## Signed fact

Kind run.withdraw under existing hn/0; standard actor metadata/chain fields,
Subject=full request event ID, Body=trimmed nonblank reason <=4096 UTF-8 bytes.
Reject extraneous fields/unknown or duplicate JSON keys for this new kind only.
The request must exist, be run.request and have the same actor/public key and
earlier sequence. Shared local/quarantine/archive validation enforces the same
relationship; one request cannot acquire conflicting duplicate withdrawal facts.
Old signed bytes are unchanged and old-reader rejection is disclosed.

## Scheduling

Use existing verified lineage states and exact pipeline evidence semantics.
Never count untrusted, unauthorized or mismatched results. A failed latest result
from a runner replaces its prior pass. Empty/absent policy pipeline leaves old
explicit runner execution possible. Skip diagnostics are not signed run results.

## Snapshot target observation

Only explicit stale-base mode reads the exact configured remote branch.
It uses bounded/cancellable noninteractive Git transport, no local ref writes.
Uncertain observation prevents backend start. Movement after an observation or
another runner starting from the same observed facts remains possible.
