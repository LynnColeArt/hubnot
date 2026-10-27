# Native observational audits v1

Audits inspect any already-present Git commit, including old or merged code,
without a proposal. They are signed observations. They never satisfy proposal
reviews, required pipeline results, acceptance decisions, or merge evidence.
Requests, imports, and reads do not execute target/rule-source hooks, pipeline
steps or foreign workflow definitions, or fetch dependencies. Actual signed
fact/ref writes retain existing destination/operator `reference-transaction`
hooks. An operator hook can reject a publication without changing accepted ref
visibility; inspection and replay perform no new write. The private uncommitted
request operation guard suppresses hooks because it publishes no fact.

## Explicit authority

An optional `.hn/policy.json` member enables audit roles:

```json
{
  "audits": {
    "scope": "my-project",
    "ruleOwners": ["FULL_64_HEX_ACTOR_FINGERPRINT"],
    "requesters": ["FULL_64_HEX_ACTOR_FINGERPRINT"],
    "executors": ["FULL_64_HEX_ACTOR_FINGERPRINT"]
  }
}
```

Each role requires 1–64 distinct full actor fingerprints. Scope is a nonempty
identifier of at most 128 bytes. Policy inspection and comparison display scope,
enablement, and each role change. Missing `audits` disables new work; it does not
erase historical observations. The sole owner can authorize their own rule.
Workflow publication, display names, Git author fields, and foreign reviews grant
no audit role.

The operator must separately adopt a named local branch. The private selection
is `.git/hn/audit/authority.json` beneath the resolved Git directory, in protected
private directories with a regular mode-0600 file. Symlinks, unsafe permissions,
symbolic aliases, and selectors other than direct `refs/heads/NAME` are refused.
Selection updates use an exclusive lock and atomic replacement, guarded by
`absent` or the SHA-256 digest of the exact prior selection bytes. The file
contains only `{"ref":"refs/heads/main"}`. The selection digest stays stable
when that branch advances; inspection also resolves its current commit and exact
policy digest. This file is not a collaboration fact or replication resource.

There is no implicit HEAD authority. Initialization, branch checkout, imported
facts, and candidates never install or change the selection. Every writer and
execution check resolves its current branch independently. An optional policy
revision assertion must resolve to that exact current commit; it cannot select a
candidate or revive withdrawn authority. A branch advance with identical policy
bytes preserves request eligibility. Policy-byte changes, role withdrawal,
revoked/conflicting rules, and stale authorization IDs prevent new work.

Current rule projection additionally requires each recorded policy commit to be
an actual ancestor of the adopted branch. Matching scope, rule ID, actor, or
policy bytes alone are insufficient. A foreign or forked policy history cannot
introduce current authorization, conflict, or revocation heads. These facts can
still be valid historical observations. Snapshot packs do not prove ancestry.

## Signed schema

The existing `hn/0` envelope gains one optional `audit` object. Existing events
omit it and retain their signed encoding. Audit kinds reject unrelated envelope
fields, unknown/duplicate JSON keys, invalid Unicode, unrelated per-kind fields,
and payloads larger than 2 MiB. `operation` is allowed only on audit requests
(up to 256 bytes); actor-scoped retry orchestration is a caller responsibility.
Every audit payload includes:

- `scope`, `rule`: bounded project and stable rule identifiers; rule uses the
  existing pipeline-name syntax (1–64 bytes).
- `policyCommit`: exact resolved historical Git commit.
- `policyDigest`: SHA-256 ID of the exact historical policy bytes.

Additional fields by kind:

| Kind | Additional members |
| --- | --- |
| `audit.rule-authorize` | `parents` (0–64 full predecessor IDs), `source`, exact UTF-8 `definition`, `definitionDigest` |
| `audit.rule-revoke` | `parents` (1–64), nonempty `reason` (up to 4096 bytes) |
| `audit.request` | `authorization`, `codeCommit`, `definitionDigest`, nonempty `intent` (up to 4096 bytes) |
| `audit.result` | `authorization`, `codeCommit`, `definitionDigest`, `request`, `outcome`, `findings`, `log`, `backend`, `platform`, `runner`, `exitCode`, `durationMs` |

Zero numeric values and empty optional arrays can be omitted. Exact default
Go JSON encoding is signed; validation always authenticates original bytes.
Historical validation uses recorded policy objects, signer membership, exact
predecessor references, rule/definition binding, and request/result binding. It
needs no current local anchor. A result can record a later authority commit
with identical policy bytes; exact code, authorization, rule, digest and requester
membership must still match. Local writers also recheck current authority
immediately before appending to the signing actor's stream.

Authorizations and revocations form explicit predecessor graphs. Writers require
all current heads as their expected predecessors. Concurrent independently signed
heads remain visible and block execution; no timestamp or cross-actor sequence
wins. Reauthorization is an explicit successor. Old requests do not automatically
bind to the successor.

### Rule sources

`source` is one of these closed shapes:

```json
{"kind":"pipeline","commit":"RESOLVED_OID","path":".hn/pipelines/check.json"}
{"kind":"git","commit":"RESOLVED_ALREADY_PRESENT_OID","path":"rules/check.json"}
{"kind":"workflow","revision":"sha256:FULL_NATIVE_REVISION_ID"}
```

Git sources must be regular committed blobs at safe relative paths. The captured
bytes must exactly match `definition`, which is a validated bounded native
`hn.pipeline/0` document. Native workflow authorization selects an exact current,
unconflicted, enabled runnable revision. Disabled, foreign-only and conflicted
workflows refuse authorization. Later workflow edits do not silently replace an
already authorized rule; an owner must explicitly authorize a successor.
There is no implicit fetching, YAML execution, or dependency installation.

### Results and findings

Outcome is `passed` with exit code 0 or `failed` with exit code 1–255. Duration
is nonnegative. Backend is `sandbox` or `host`; platform and runner are nonempty
bounded provenance strings (128 and 256 bytes). `log` authenticates exact
`log.txt` bytes, limited to 1 MiB.

`findings` contains at most 128 inert objects. Each has a unique stable `key`
(identifier, up to 128 bytes), nonempty `title` (512 bytes), optional `body`
(16 KiB), optional safe relative `path` (1024 bytes), and optional nonnegative
`line`. Positive line numbers require a path. Findings do not execute anything
or independently open issues. Execution and explicit follow-up behavior use
these foundation types but belong to the audit command layer.

## Snapshot dependency transport and admission

Audit event trees allow exactly regular `event.json`, `signature`, `audit.pack`,
and, for results only, `log.txt`. The pack is inert transport, not complete Git
history and not authority. Its exact declared object set contains:

- The recorded policy commit, root and intermediate trees proving the policy
  path, and exact `.hn/policy.json` blob.
- For a Git rule source, its commit and regular source-path proof trees/blob.
- For a request or result, the exact target commit and its complete snapshot tree.

Commit parents are intentionally not traversed. Gitlinks remain tree metadata;
submodule content is not invented. Repeated objects in one pack occur once.
Native workflow revisions travel as exact signed event dependencies. The separate
whole-project migration format carries full Git history; `audit.pack` must not
be represented as a full-history bundle.

Packs use Git pack version 2 with nondelta commit/tree/blob objects. Limits are
64 MiB compressed, 100,000 objects, 16 MiB per expanded object, and 128 MiB total
expanded bytes. The parser checks expansion, checksum, count, object types and
hashes. Both local writes and imported reads first use isolated temporary object
storage with Git's `index-pack --fsck-objects` syntax checks (including required
author/committer headers, identities, dates and tree encoding/order). Independent
object-type lookup verifies commit-tree links, included parent links and tree
entry modes against their actual objects; the type printed by `ls-tree` alone
is not sufficient. Exact snapshot/path closure requires target trees and source
proof members while allowing omitted parent history, unrelated proof-tree
members and gitlink content. It also verifies regular source paths, safe tree
members, and absence of injected extra objects before
installation into the selected object database. Fetched data uses quarantine;
no signing-owned ref, branch, policy, worktree or private anchor is changed.
Malformed/unknown event tree members, missing packs, corrupt packs, extra pack
objects, oversized logs, missing event dependencies, and invalid historical
permissions refuse accepted visibility.

Existing per-selection replication limits also count expanded dependency objects
and bytes before unpacking. Repeated objects within an actor history count once;
only the previously accepted history is discounted. Default transport attachment
limits may be lower than the protocol pack ceiling; the operator must explicitly
choose suitable ordinary replication budgets for a larger snapshot. This first
format repeats snapshot material between facts, favoring bounded verifiability
and ordinary actor-ref transport over an implicit new ref namespace.

## Foundation API and command integration

The foundation exposes `AuditAuthoritySet/Show`, `AuditAuthorize`, `AuditRevoke`,
`AuditRequest`/`AuditRequestWithIntent`, `ValidateAuditExecution`, and
`RecordAuditResult`. `AuditExecution` returns separately authorized definition
bytes through its authorization plus the validated pipeline and target request.
`RecordAuditResult` records a supplied completed execution and rechecks authority;
it never runs code. The command layer wires `hn audit authority set/show`, rule,
request, execute, list/show/logs and explicit follow-up operations to these APIs.
It retains sandbox defaults and per-invocation unsafe-host opt-in. There is no
scheduler, batching worker or required background process.

## Explicit native commands

The compiled CLI self-registers `hn audit`; registration and `hn init` perform
no audit setup or execution. All audit commands support `--json` (`hn.audit/1`,
`ok`, `data` or `error`). Mutation responses expose the full signed fact `id` and
`event`; failures return a nonzero exit and an `audit_refused` error. The active
signer can be checked with optional `--actor FULL_FINGERPRINT`. Arguments must
be valid UTF-8; invalid bytes refuse before private intent or fact writes. Reads require no
private signer, including when audit opt-in has since been withdrawn.

```text
hn audit authority set --ref refs/heads/main --expect absent
hn audit authority show
hn audit rule authorize check --source pipeline --at RULE_REV
hn audit rule authorize check --source git --at ALREADY_PRESENT_REV --path rules/check.json
hn audit rule authorize check --source workflow --workflow FULL_NATIVE_REVISION
hn audit rule list
hn audit rule show check
hn audit rule revoke check --expect FULL_AUTHORIZATION --reason "Retired"
hn audit request --at CODE_REV --rule check --intent "Inspect released code" --operation release-audit
hn audit execute FULL_REQUEST --backend sandbox
hn audit show FULL_RESULT
hn audit logs FULL_RESULT
hn audit follow-up FULL_RESULT --finding missing-test --operation triage-release
```

A pipeline source defaults to `.hn/pipelines/RULE.json`; `--path` may select a
safe committed regular blob explicitly. Git sources require `--path`. Workflow
sources require one exact enabled, unconflicted native runnable revision and
accept neither `--at` nor `--path`. None of these commands fetches a missing
revision. Rule successors require every exact current `--expect` head. Rule
list/show includes historical authorizations and revocations; show additionally
reports `current_heads` when the local authority is available. History is never
used to infer an authority anchor. `audit list`, `rule list`, and `rule show`
page sorted full IDs with `--limit 1..200` (default 50) and `--offset N`; the
response includes `total` and `next_offset` (0 at the end). Offsets are over the
current verified history, so restart pagination if new facts arrive.

`request`, rule authorization/revocation, and `execute` accept optional
`--policy-ref REV` solely as an assertion of the adopted branch's exact current
commit. Targets and Git rule sources resolve to immutable commits at invocation.
They may differ from each other and from the current authority branch. Request
never runs code. Execute uses only the exact authorized definition, extracts the
request's target snapshot with the existing runner, and rechecks authority both
before execution and before signing the result. It preserves the runner's fixed
environment, step timeouts and sandbox default. Selecting `--backend host` also
requires `--allow-unsafe-host-execution` on that invocation. It never schedules,
merges, pushes, syncs, installs dependencies or opens issues.

A passing or failing pipeline records an observational result and exits the CLI
successfully; inspect `event.audit.outcome` and `exitCode`. A runner launch or
transport failure, invalid structured output, or authority withdrawal produces
an error with no result publication. Normal command exits and step deadlines
remain pipeline failures. Existing sandbox startup diagnostics retain the shared
runner's behavior; a nonzero backend process exit is observable as a failed run.
Logs authenticate at most 1 MiB, including any truncation notice. A failed audit
neither blocks native proposal readiness nor supplies any required CI/review or
merge evidence. Existing native policy and required checks continue independently.

### Request operation recovery

An optional request `--operation` is actor-scoped (up to 256 bytes). Repeating it
with the same resolved code, rule and intent returns the original signed request;
changed intent refuses. Successful recovery reads verified signed facts, even if
the acknowledgement was lost. Without a policy assertion this is historical
replay, so it can return an old request after withdrawal without authoring new
work. A supplied policy assertion is still checked against current authority.

CLI invocations using the same repository/actor/operation serialize verified
lookup through append with a prepared, uncommitted Git verify transaction under
`refs/hn/local/audit-request/ACTOR/OPERATION_HASH`. No guard ref is committed or
replicated; guard acquisition disables repository reference-transaction hooks.
Contention returns a clear retry error. Closing the client pipe aborts the Git
transaction, including ordinary client termination. Hard termination of the
entire process group or power loss can leave an ordinary Git `.lock` file:
confirm no previous writer remains before applying normal Git stale-lock
recovery, then retry the original intent. This CLI guarantee does not add
idempotency to direct low-level API calls or provide distributed exactly-once
publication from independent replicas of the same signing actor.

## Structured findings

A pipeline can emit one finding per stdout/stderr line, prefixed exactly with
`HN_AUDIT_FINDING `, followed by a strict JSON object:

```text
HN_AUDIT_FINDING {"key":"missing-test","title":"Add coverage","body":"Observed gap","path":"src/check.go","line":12}
```

Only the capped authenticated log is parsed. Unknown or duplicate JSON fields,
invalid JSON, normalized duplicate keys, invalid paths/locations, excessive
lengths or more than 128 findings refuse result publication. Keys normalize by
trimming surrounding Unicode whitespace and Unicode lowercasing; identity uses
the normalized key. Titles, bodies and paths otherwise retain their content.
The signed schema limits above apply. A truncated structured record is invalid;
keep structured records near the beginning of bounded output. Findings are inert
text and never grant authority or execute anything. A failed pipeline without
structured findings produces one stable `rule-failure` finding, linked to its
bounded raw result log. Passing pipelines without findings produce none.

## Deliberate follow-ups and interruption recovery

`hn audit follow-up FULL_RESULT` explicitly processes that result's findings;
`--finding KEY` selects one normalized key. It uses ordinary native signed issue
writes and never changes an existing issue. Finding identity is the SHA-256 of
the standard compact JSON array `[scope, stableRuleID, normalizedFindingKey]`,
independent of timestamps, result ID, target revision or changing title/body.
Issue opening metadata records `audit/finding` (that digest), `audit/request` and
`audit/result` (the exact full signed IDs). Inspection authenticates the opening
links against verified audit facts and their findings. A later issue edit or
removal of current metadata does not erase its original association.

Before creating anything, follow-up validates the requested result/selection.
Historical signed results may contain raw-unique keys that normalize to the same
finding identity. Follow-up checks the complete selected finding set and refuses
any such ambiguity before writing private intent or an issue, including ambiguous
`--finding` selections. Choose an unambiguous finding or use a new result with
distinct normalized keys; original signed finding bytes remain unchanged.
It then checks prior authenticated opening links and binds an actor-scoped operation to
its exact original result and selected finding identities. `--operation` accepts
1–128 printable non-whitespace ASCII bytes; its default derives from that result
and selection. Protected private intent files below the resolved Git directory
(`hn/audit/followups/ACTOR/OPERATION_HASH.json`, and request intents under
`hn/audit/requests/ACTOR/OPERATION_HASH.json`) use atomic expected-absent installation
and mode 0600. They are recovery bindings, never policy authority or an issue
index, and are excluded from replication. Unsafe symlinks/permissions refuse.
Retain intent files for operations that have not completed.

Each create also uses a deterministic actor-scoped native issue operation key.
After interruption or an uncertain acknowledgement, retry the same result,
finding selection and operation. Verified signed issue history recovers each
completed create; remaining findings use their original snapshot. A changed
result or selection with the same operation refuses before creating more work,
including when the original operation only returned existing associations.
Private lookup caches are unnecessary; authenticated opening records remain the
portable dedup source. A retry returns existing associations and preserves all
operator edits, comments, status changes and issue conflicts.

The response lists `associations` with finding key, stable `identity`, `issue_id`,
original `result`, and `existing`. Multiple observed associations for one identity
set `multiple_associations`; inspect and reconcile explicitly. Independent
publishers or concurrent distinct operations can create multiple signed issues
before observing each other's links. There is no distributed exactly-once claim,
no cross-actor lock and no automatic deletion or reconciliation. Execute/follow-up
perform no network access; sync selected actor histories explicitly to exchange
results and issue links. A keyless recipient can inspect all historical audit
facts/logs, while new execution requires deliberate local authority adoption and
a currently authorized signer.
