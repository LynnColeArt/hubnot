# Native observational audits v1

Audits inspect any already-present Git commit, including old or merged code,
without a proposal. They are signed observations. They never satisfy proposal
reviews, required pipeline results, acceptance decisions, or merge evidence.
Requests, imports, and reads do not execute repository code or fetch dependencies.

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
hashes. Isolated temporary object storage verifies exact snapshot/path closure,
regular source paths, tree members, and absence of injected extra objects before
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
