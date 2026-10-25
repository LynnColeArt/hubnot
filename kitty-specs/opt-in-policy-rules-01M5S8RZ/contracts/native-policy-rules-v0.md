# Native policy rule contract v0

This reference describes the implemented optional extensions to `hn.policy/0` and the native CLI. The maintained operator guide is `docs/governance-v0.md`; the signed event format remains `hn/0` in `docs/protocol-v0.md`. No new event kind or provider authority is introduced.

## Configuration and defaults

The existing required policy fields retain their meaning. These optional top-level fields are independent:

| Field | Members | Activation |
|---|---|---|
| `protectedChecks` | `paths`, `trustedReviewers`, `requiredApprovals` | Omitted or null is disabled; a valid object enables protection. |
| `automerge` | nonempty `rules` | Omitted or null is disabled; a valid object enables named-rule eligibility. |

Every automerge rule contains `name`, `targetBranch`, `actors`, `authors`, and `paths`. Actors are full lowercase 64-hex native Ed25519 public-key fingerprints. Executing actors must also be maintainers in the same base policy. Authors are explicit native proposal actors, with no imported username or identity-continuity substitution.

Protection paths are an explicit array, which may be empty because enabled protection also covers `.hn/policy.json` and every required `.hn/pipelines/<name>.json`. Designated reviewers are nonempty and the approval threshold is between one and their count. Rule actor/author/path arrays are explicit and nonempty. New arrays and rule counts are capped at64, literal paths at4096 bytes, and policy bytes at1MiB. Names and branches pass native syntax validation. Invalid/unknown fields fail policy validation.

Paths are literal canonical repository-relative files or directories ending in `/`. Absolute/traversal/control-character/backslash/colon/wildcard paths are rejected. Protection includes additions, deletions, renames, mode/type changes, descendants of selected directories, and replaced ancestors. Auto allowance includes only exact listed files or descendants of listed directories; a descendant permission cannot authorize changing an ancestor.

Initialization, upgrades, transfer commands and ordinary reads do not add either section. Older clients reject these unknown fields even when null; omission preserves compatibility for unconfigured repositories.

## Protected evidence and prospective merge

The signed proposal base supplies exact policy bytes and digest. Current exact-candidate approvals from distinct designated actors other than the proposal author qualify separately from ordinary reviewer membership. A designated review may also count as ordinary approval only when independently trusted there. The client includes exact qualifying IDs in acceptance evidence; local and quarantine admission share the evidence validator. Editing/removing the section in the proposed head cannot authorize itself.

A new enabled manual merge rechecks current evidence and refuses pending designated approvals or withdrawal. It requires a revision when the current protected target policy digest differs from the candidate base policy, or protected paths changed from signed base to current target. This closes a head that incorporates a tightened target and restores an older lint file while its base-to-head diff appears harmless. Unprotected target movement remains available to manual merge. Immutable historical decisions and contained merge-event repair remain governed by their signed base.

## Automatic command

```text
hn automerge PROPOSAL --rule NAME
hn automerge PROPOSAL --rule NAME --execute
```

`PROPOSAL` is a complete native `sha256:<64-hex>` event ID. The rule is explicitly selected; no fallback or implicit match executes. Default preview verifies candidate eligibility, prints exact code/policy/target identities, permitted executing actors, quoted changed paths and blockers. It does not load/migrate a private key, append an event, update the index/worktree/refs, request a run or publish anything. Eligible preview exits0; blocked/invalid preview returns nonzero. Candidate eligibility is distinct from authorization of a selected executing signer.

Both modes first refuse configured partial/promisor repositories, shallow history and unfinished Git operations using bounded metadata inspection, before object readers can lazily fetch or write shallow recovery records. Execution needs a full locally available repository, a clean named target, target OID equal to signed base, locally verified descendant head and matching signed proposal code ref. Every changed path, author and full target ref must match the selected base-policy rule. Existing current review, CI and maintainer acceptance requirements remain; current trusted failures/change requests/rejections veto automation. Empty changes, policy amendments, and enabled-protected changes require the manual path. The actual common signing identity must match the authorized selected rule actor.

Execution rechecks current facts under a prepared expected-old Git ref transaction carrying the signed base through the actual ref update. The held lease has no whole-operation expiry; acknowledgements and child exit have finite waits. The automatic merge uses the signed head tree with first parent base and second parent head, safe checkout, ignored-file collision preflight and actual-ref observation on uncertain acknowledgement. It creates the existing signed `proposal.merged` event with exact candidate/head/result commit/base-policy digest/current acceptance IDs. It does not manufacture approvals or acceptances, run pipelines, sync, push or start a daemon.

## Recovery and authority boundary

If code merged but fact recording failed, `hn merge PROPOSAL` uses existing unique direct-parent merge discovery and current evidence to record the missing fact without another code merge. Ambiguous or stale evidence refuses. Checkout/transaction uncertainty preserves inspectable state and avoids blind hard reset; callers inspect local edits rather than assuming an error means no commit happened.

Required checks are configured through native pipeline policy. Commit-tree does not execute commit/pre-merge hooks; operator-configured Git reference-transaction hooks retain normal Git behavior. Rules contain no executable expression language. File protection does not semantically recognize arbitrary lint weakening, cover unlisted indirect inputs, prove honest CI execution, or control raw Git/host/keyholder actions outside Hubnot. Fact and branch checks are observed local snapshots, not a distributed global transaction.

## Verification trace

FR-001 through FR-010 map to `../validation-report.md#functional-evidence`. Public CLI acceptance exercises normal and nogithub binaries, separate identities and real bare Git, with controlled Git-only child PATH. Implementation/runtime review source is38a72003 and final public verification source is250224e8; exact full hashes and release results are recorded in the validation report.
