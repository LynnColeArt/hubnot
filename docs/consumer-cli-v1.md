# Native consumer CLI v1

The native consumer contract is optional. Discover actual supported capabilities
before selecting machine output:

```sh
hn version --json
hn --version --json
hn capabilities --json
```

These equivalent static commands work outside a repository without identity,
Git, authority selection, contribution locks, extension registration or optional
module bootstrap. `hn version` and `hn --version` retain their original text
bytes. JSON discovery accepts only `--json` (or `--json=true`) and no positional
arguments. Unknown flags return `native.invalid_input`, exit 1.

Success has exactly the envelope `{ok:true,data:...,error:null}`; failure has
`{ok:false,data:null,error:...}` and exit 1. Successful discovery data contains
`schema: "hn.consumer/1"`, the binary's `version`, and a `capabilities` object.
A capability is supported only when present; consumers must not infer support
from the binary version or from this document's future additions.

Current capability entries:

| ID | Version | Meaning |
|---|---|---|
| `native.discovery` | 1 | The three static commands above. |
| `contribution.existing-json` | 1 | The pre-existing contribution output described below; it does not use the consumer envelope. |

All errors use the existing closed native code inventory in
[extensions.md](extensions.md#closed-errors-and-immutable-recovery).
The error object has exactly `origin`, `code`, `message`, `recovery`, `operation`.
Messages are sanitized UTF-8 and at most 4096 bytes. This contract's unkeyed
commands return `operation:null` and `recovery:"do_not_retry"`: inspect the
outcome before making a new explicit user intent. JSON receipts add no operation
key, lost-acknowledgement recovery or automatic mutation retry promise. A failed
command also emits a generic CLI diagnostic on stderr; parse stdout for the
machine envelope.

## Existing contribution output audit

`contribution_commands.go` has nine `--json` flag registration sites. They are
compatibility flags, since their successful data output already uses
`contributionPrint` regardless of the flag. They do **not** select `hn.consumer/1`
or the `{ok,data,error}` envelope. Errors keep their existing CLI behavior.

| Registration site | Commands/aliases | Existing output |
|---|---|---|
| authority set | `contribution authority set` | Selected authority object. |
| unselected restore | `contribution restore` without selected authority | Native intake object. |
| receive/restore | `contribution receive`, `contribution restore` | Native intake object. |
| retained | `contribution retained show`, `contribution retained register` | Snapshot/registration object. |
| intake | `contribution intake` | Receiver-local attempt, ledger and descriptor. |
| inspect | `contribution inspect` | Existing view or selected record/raw payload. |
| archive | `contribution archive` | Existing archive descriptor. |
| prepare | `contribution prepare` | Existing submission/artifact descriptor. |
| raw inspect | `contribution inspect --raw` | Exact immutable native/contribution payload and signature. |

Several registration sites serve aliases or subcommands; nine sites does not
mean nine distinct CLI command names. Other contribution commands may also emit
JSON through their existing handlers. This is a consistency/discovery audit,
not an envelope transition: no contribution default output bytes, signatures,
accepted-history selection or authority change here.

## Exact proposal reads

Capability `native.proposal-read` version 1 supports:

```sh
hn proposal show PROPOSAL --json
hn proposal status FULL_PROPOSAL --json
```

Options follow the proposal argument. `show` retains its existing read-only prefix
lookup; `status` still requires a full proposal event ID. Neither command treats a
review event ID as a proposal. JSON show evaluates the candidate like status and
therefore requires code, policy and pipeline dependencies that its original text
view may not require. Existing text handlers and output are unchanged.

Both JSON reads return the same `hn.consumer/1` data projection:

| Field | Meaning |
|---|---|
| `id`, `kind`, `actor`, `actor_name`, `title`, `body`, `base`, `head`, `policy` | Full candidate event, author fingerprint, Git revisions and base-policy digest; display names are labels. |
| `current_reviews` | Object keyed by full actor fingerprint. Each row has full event `id`, `actor`, `actor_name`, `sequence`, `verdict`, `subject` and `policy_eligible` / `protected_policy_eligible` flags. |
| `readiness` | Existing native evidence readiness, independent of lineage and current change requests. |
| `requirements` | Code availability/match; ordinary reviewer pool/eligible actors, required/actual approval count and deficit; pipelines with required passes, deficit, trusted runners/requesters and full passed/failed event references; optional protected-check paths/changed paths, count, eligible actors and evidence. `satisfied` and `missing` name the readiness requirements. |
| `evidence` | Exact full event IDs in the current native evaluation. This is current status, not the immutable evidence stored in an older decision. |
| `lineage` | Full root/predecessor/successor/sibling/ancestor/descendant/member/merge identities and merged/superseded/closed/conflict facts. |
| `disagreements` | Observed approvals/change requests, qualified change requests and whether differing current verdicts coexist. |
| `decisions` | Full current qualified accept/reject/merge event IDs, required accepts, deficit and native accepted/rejected/merged observations. |
| `status` | Existing native status, including superseded and lineage closed. |
| `observed_preconditions` | Selected-history evidence/lineage/decision preconditions described below. |

Current reviews use the existing latest actor-local sequence for **exactly** the
candidate. A late review of a predecessor remains attached to that predecessor;
no successor inherits it. The visible operative history selection preserves
contribution admission and visibility boundaries. Observed untrusted reviews
remain attributable in this selected history; qualification flags distinguish
them from policy-counting evidence. `policy_eligible` describes the actor/author
rule, not whether its verdict is an approval. Protected-check eligibility always
excludes the candidate author and uses its own designated reviewer pool.

Reviewer pools and thresholds are policy facts, **not Fleet assignments**.
Consumers join their own assigned-reviewer list with `current_reviews` to compute
pending assigned reviewers. A threshold deficit does not name an absent assigned
reviewer or introduce unanimity. Arrays/objects are empty `[]`/`{}` rather than
null; `requirements.protected_checks` is null when not enabled.

`observed_preconditions.scope` is `selected_history_evidence_and_lineage_only`.
Its `lineage_terminal` excludes superseded, closed and conflicting lineages.
`acceptance.met` combines this with readiness and not-already-merged;
`merge.met` also requires accepted and not rejected. These are scoped observed
preconditions, **not permission to write or a merge-success guarantee**. A
proposal binds no target branch; keyless reads neither identify the active
signer nor probe target policy, fresh remote state or local target advancement.
Writers recheck their existing actor, target, trust and concurrency guards.
A superseded proposal can thus have `readiness:true` and
`observed_preconditions.acceptance.met:false` without contradiction. An accepted
proposal can later lose readiness after a newer review. Change requests are
reported independently and do not silently change native acceptance policy.

## Full signed log and ordinary event resolution

Capability `native.log-read` version 1 adds `hn log --json`. It returns
`schema:"hn.consumer/1"`, `history_scope:"raw_signed_history"` and `events:[]`.
Each row includes full `id`, full storage `commit`, exact signed `event` object
and raw unpadded Base64 `signature`. This is the existing **raw signed history**
boundary: it includes unadmitted/hidden native facts, as the original text log's
explicit raw banner states under selected contribution authority. It does not
claim those facts are admitted, visible or policy-qualifying. Non-JSON log bytes
are unchanged. Empty history returns `events:[]`.

Capability `native.event-read` version 1 supports:

```sh
hn event resolve PREFIX [--json]
hn event show FULL_EVENT [--json]
```

Both accept a full event ID or 12–64 lowercase hexadecimal characters, with
optional `sha256:` prefix. Resolution invokes the existing native event resolver;
it never chooses among ambiguous matches. `ambiguous_id` and `not_found` are
separate native error codes. `resolve` without JSON prints the full event ID and
newline; `show` without JSON prints the full ID, kind, actor and signed payload.
With JSON, either returns the same complete row as log plus
`schema:"hn.consumer/1"` and `history_scope:"visible_operative_history"`.

Event reads verify native selected history and preserve existing shallow and
pending-history refusal boundaries. Ordinary event selection applies existing
contribution admission/visibility. Reviews, decisions, merges and run requests
require their proposal candidate to remain selected; run results and withdrawals
require their request, and hence its candidate, to remain selected. This prevents
hidden or unadmitted candidate evidence bodies from leaking through event reads.
A visible successor is not hidden merely because its predecessor is hidden:
a revision's `subject` is immutable lineage, not a new authorization-parent rule.
The separate raw log retains all verified native facts explicitly.

These reads append no signed facts, load no signer and migrate no legacy private
identity. Prefix lookup grants no mutation authority. Proposal status still takes
a proposal ID; use event read/resolve separately for review/result evidence IDs.
Every existing mutation still requires full IDs, and artifact's documented
no-prefix rule is unchanged. No artifact command or signed wire bytes change.
