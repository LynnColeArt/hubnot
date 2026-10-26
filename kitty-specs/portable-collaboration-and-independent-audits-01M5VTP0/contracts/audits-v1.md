# Independent audit contract v1

## Opt-in authority

An optional committed `audits` policy contains a stable project `scope` and
bounded nonempty full-actor lists `ruleOwners`, `requesters`, `executors`.
Absence preserves old behavior and disables new authoring/execution, while
historical records remain readable. Policy inspection/diff exposes all changes.

The operator explicitly adopts a private local authority anchor with
`hn audit authority set --ref refs/heads/main --expect absent` (or an exact prior
anchor digest for an update). `show` reports its canonical named local branch and
current resolved policy commit/digest. Store it as a protected regular private
file below the resolved Git directory (`hn/audit/authority.json`), with
expected-old guarded atomic updates. Refuse stale updates, unsafe links and
non-branch authority selectors. This local control is not a signed collaboration
fact and is excluded from snapshots/replication.

There is no implicit HEAD default. Initialization, imports, candidates, branch
checkouts and observed signed records never adopt/change the anchor. With no
anchor, audit authoring/execution refuse and explain explicit adoption; historical
reads continue. Authoring/execution independently resolve the anchored current
branch even when the checked-out branch, target or rule source differs.
`--policy-ref` is only an optional assertion matching the anchored current
selection; it cannot override that branch or resurrect an old withdrawn policy.

Record scope, historical resolved policy commit and digest in signed facts.
Execution requires current policy digest, owner/requester/executor membership and
current unconflicted authorized rule heads. A code-only advance of the anchored
branch with byte-identical policy does not invalidate a request by commit alone.
Historical validation uses recorded policy objects, not today's private anchor.
A candidate, imported rule or collaborative workflow cannot choose its own owners.

Current rule-head projection also isolates the selected authority history:
recorded authority-policy commits must belong to the anchored branch's accepted
Git lineage, alongside the current policy/role checks. A foreign or forked policy
claiming the same public scope/rule identifiers cannot introduce current conflict
or revocation heads. Such otherwise valid historical observations remain
inspectable outside the active authority projection. Test this poisoning attempt.

The sole designated owner may authorize their own rule. Do not apply the
protected-check nonauthor exclusion to this owner signature. Git author fields,
display names, workflow publication and foreign reviews are not authorization.

## Signed facts and lifecycle

| Kind | Binding |
| --- | --- |
| `audit.rule-authorize` | Scope, stable rule ID, authority policy, inert source/revision, exact validated definition/digest and predecessor heads |
| `audit.rule-revoke` | Same scope/rule, authority policy, exact predecessor heads and nonempty reason |
| `audit.request` | Scope/policy, resolved code commit, exact rule-authorization ID and requester intent |
| `audit.result` | Request and exact code/rule binding, executor, outcome, bounded findings/log digest and backend/platform provenance |

Use strict typed schemas within the existing envelope; do not change old signed
encodings. Add these kinds consistently to event validation, local storage,
quarantine, shallow dependencies and archive admission. Explicitly allow only
their declared bounded log/tree members. Audit facts never count as proposal
review, `run.result`, acceptance or merge evidence.

Authorizations/revocations form expected-head revision graphs per scope/rule.
Concurrent heads remain visible; execution refuses conflict or revocation.
Reauthorization is an explicit owner-signed successor. Never use timestamps or
cross-actor sequence numbers to pick a winner. Current policy change, owner
withdrawal, revoked/conflicting rule heads or stale request binding prevent new
execution. Preserve historically valid authorization/result facts under their
recorded policy; later changes do not erase or retroactively invalidate history.

## Sources and execution

Initial sources: committed local pipeline, exact native workflow revision, or
explicit already-present external Git revision/path. Capture canonical runnable
bytes/digest and source provenance at authorization. Foreign-only, disabled or
conflicted workflows are not implicit runnable definitions. No automatic fetch,
foreign YAML execution or dependency installation.

Requests can inspect old or already-merged code without a proposal. Authority
comes from the selected current policy, not the old target's policy. Retain
target/rule dependencies through replication and offline migration.

Reuse low-level pipeline execution, supplying separately authorized definition
bytes and extracting target code. Preserve sandbox default, explicit unsafe-host
flag, fixed environment, step deadlines and capped logs. Request/import never
executes hooks or code. Execute does not approve, merge, push, sync or open issues.
There is no scheduler, batching worker or required background process.

## Proposed explicit CLI

```text
hn audit authority set --ref refs/heads/main --expect absent|ANCHOR_DIGEST
hn audit authority show
hn audit rule authorize RULE --source KIND --at REV [--path PATH|--workflow ID] [--expect AUTH]... [--policy-ref REV]
hn audit rule revoke RULE --expect AUTH... --reason TEXT [--policy-ref REV]
hn audit rule list|show RULE
hn audit request --at REV --rule RULE [--policy-ref REV] [--operation KEY]
hn audit execute REQUEST [--policy-ref REV] [--backend sandbox|host] [--allow-unsafe-host-execution]
hn audit list|show|logs ID
hn audit follow-up RESULT [--finding KEY] [--operation KEY] [--json]
```

Finalize flag spellings consistently across docs/public tests. Reads need no
private signer. Mutation IDs are full native IDs and actor expectations follow
existing CLI conventions.

## Findings and follow-up

Define bounded stable finding keys/title/body/path/location and a documented
structured format. An unstructured failing pipeline still yields one stable
rule-level failure with its bounded raw log. Findings are inert content.

Dedup identity is scope + stable rule ID + normalized finding key, not timestamp
or only result digest. Signed issue metadata (`audit/finding`, `audit/request`,
`audit/result`) retains links. Inspect prior links and use actor-scoped native
operation guards before opening new work. Repeated processing returns the
existing association; it does not silently overwrite an operator-edited issue.
Concurrent independent publishers may produce multiple visible associations;
report them and reconcile explicitly. Private indexes are disposable projections.

Acceptance must cover disabled defaults, sole-owner authorization, refused
candidate self-authorization, collaborative/imported workflow authority refusal,
stale `--policy-ref`, imports without a local anchor, deliberate authority adoption,
unchanged-policy code advances, current revocation/conflict/stale policy,
historical readability, nonblocking
failures, rejected audit-as-merge evidence, follow-up retry and offline replication.
