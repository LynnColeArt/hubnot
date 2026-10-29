# Contribute without upstream write access

Hubnot can receive signed issues, replies, discussions and code proposals as
local files. The contributor needs Git, `hn` and their own native identity.
They need no provider account, upstream push permission or project role. The
maintainer explicitly receives the file; enabling this workflow starts no
listener, worker, scheduled job or network request.

Contributions are independently optional. Without selected contribution
authority, ordinary native collaboration keeps its existing behavior. Protected
checks, automerge and audits remain separate opt-ins. A signature attributes
bytes to a key; it does not establish a unique person, project admission,
maintainer authority, review, passing CI, proposal acceptance or execution rights.

This guide describes the implemented first profile. The detailed record and
authority contract is in [contribution-contract.md](contribution-contract.md).

## Enable a project deliberately

Start with an already reviewed `.hn/policy.json` and a legitimate maintainer
identity (`hn identity show --json`). Choose one random 32-byte project scope
and represent it as `sha256:` followed by 64 lowercase hexadecimal digits.
Retain that identifier through project history. It identifies a moderation
domain, not a remote, branch commit or directory. An independent fork chooses
another scope; sharing it is a deliberate decision to share project moderation.

Add a `contributions` member to the native policy. The example below permits
automatic issues and replies and manual code-proposal admission. Replace
`PROJECT_SCOPE` and `MODERATOR_FINGERPRINT` with complete values. Keep the
existing `version`, `maintainers`, `proposals` and `pipelines` policy members.

```json
{
  "project": "PROJECT_SCOPE",
  "moderators": ["MODERATOR_FINGERPRINT"],
  "rules": [
    {"operation": "issue.create", "automatic": true},
    {"operation": "issue.reply", "automatic": true},
    {"operation": "discussion.create", "automatic": true},
    {"operation": "discussion.reply", "automatic": true},
    {"operation": "proposal.create", "automatic": false}
  ],
  "baseline": [],
  "limits": {
    "wireBytes": 16777216,
    "decodedBytes": 33554432,
    "objects": 4096,
    "events": 2048,
    "depth": 256,
    "milliseconds": 30000,
    "window": {"bytes": 268435456, "objects": 32768, "events": 16384, "requests": 256},
    "retained": {"bytes": 1073741824, "objects": 131072, "events": 65536, "requests": 1024}
  }
}
```

An absent operation rule denies admission for that operation. For author edits,
add an explicit `issue.edit`, `discussion.edit` or `proposal.revise` rule as
appropriate. Edits still require the original author and exact native parents.
An optional `actors` array restricts full signer fingerprints;
`"allowMaintainers": true` explicitly permits native maintainers to edit another
author's resource where the native operation permits it. This never supplies
native review, merge or execution evidence.

For an existing governed project, validate the amendment and adopt it using
the current base-policy proposal/review/CI/acceptance/merge path:

```sh
hn policy check --base main --file .hn/policy.json
# Commit on an amendment branch, then use the project's normal governance.
hn policy check --base main --head contribution-policy
hn proposal open --base main --head contribution-policy 'Enable contributions'
```

Do not replace an established policy by directly moving its authoritative
branch. Initial bootstrap is an explicit local trust decision about inspected
code and policy. Once the legitimate policy is on the chosen branch, a current
maintainer selects that direct local branch:

```sh
hn contribution authority set --ref refs/heads/main --expect absent --json
hn contribution authority show --json
```

The output identifies the selected commit, policy digest, anchor digest and
whether the view is read-only. Subsequent explicit readoption requires the
exact prior anchor digest as `--expect`. Verified descendant policy changes
follow native governance; a source archive cannot select authority for you.
Linked worktrees select their own private authority while sharing the physical
common Git directory's project accounting and operation lock.

`baseline` is a sorted finite list of exact historical native revisions to admit
at activation. Empty means that no old content is admitted just because it was
already present. Include the required authenticated dependency closure and
allow enough capacity: baseline registration is bounded and charged before
activation completes. There is no wildcard or timestamp baseline.

## Prepare and hand off an issue or reply

The following commands run in the contributor's repository. Obtain the project
scope from the maintainer. An ordinary clone does not copy a signing key or
select contribution authority. Create your own identity:

```sh
git clone /path/to/project contributor
cd contributor
hn init --name 'Contributor'
hn identity show --json
hn issue open --operation report-001 --json 'Reproduction fails on fresh clone'
```

Read the full `data.event_id` from the JSON. In the following examples,
`REVISION_ID`, `SUBMISSION_ID`, `FILE_DIGEST`, `DECISION_ID` and `ACTOR` stand for
exact returned IDs/fingerprints, never shortened display IDs. `PROJECT_SCOPE`
is the agreed project identifier.

```sh
hn contribution prepare --project PROJECT_SCOPE --revision REVISION_ID --output ./issue.hnc --json
```

Keep the returned `digest`, `submission`, `revision` and complete `refs` list.
Deliver the regular file plus those values through your ordinary file handoff.
No hosted inbox or external discovery integration is implemented or required.
The file contains original signed native facts, complete selected actor and
code/dependency closure, and an outer signed submission. It grants no authority
to run a command or fetch arbitrary refs. Preparation requires the original
native signer. It can refuse a history outside the supported outer profile.

The maintainer receives the exact inspected digest and exact returned refs:

```sh
hn contribution receive --file /path/to/issue.hnc --digest FILE_DIGEST --operation inbox-001 --ref refs/hn/actors/ACTOR --json
hn contribution pending --json
hn contribution inspect SUBMISSION_ID --json
hn issue list --json
```

Repeat `--ref` for **every** returned ref. Do not replace the list with an
arbitrary branch, wildcard or remote. `--project` is required when preparing
without a local selected authority; `--ref` selection is required at intake.
An automatic issue rule admits the exact operation after bounded authenticated
intake and a charged signed receipt. It does not require a moderator to approve
every message. A pending operation remains outside the shared view.

Replies use the same machinery. Prepare each returned revision separately;
the receiver must already admit its required parents/references:

```sh
hn issue comment ISSUE_ID --body 'Additional reproduction details' --operation reply-001 --json
hn issue history ISSUE_ID --kind comments --json
hn discussion open --title 'Design question' --body 'Question details' --category help --json
hn discussion reply add DISCUSSION_ID --body 'Possible answer' --json
hn discussion replies DISCUSSION_ID --json
# Use the returned event_id with contribution prepare, then receive as above.
```

A root reference needs the root's current heads to be visible; an exact revision
pin needs that revision visible. An old visible opening does not authorize a new
reply to a hidden current revision. Complete actor ancestry is authenticated and
retained; unsupported predecessors are never silently removed.

## Propose code and admit it separately

Commit code locally and open a native proposal with an exact base and head:

```sh
git switch -c contribution
# Edit and commit the proposed code.
hn proposal open --base main --head HEAD 'Fix the reproduction'
hn proposal list
git for-each-ref --format='%(refname)' refs/hn/proposals/
hn contribution prepare --project PROJECT_SCOPE --revision PROPOSAL_ID --output ./proposal.hnc --json
```

Use the complete proposal ID: each `refs/hn/proposals/HEX` ref names
`sha256:HEX` (all 64 hex digits). The selected local admission output also
prints the complete ID; ordinary proposal lists use shortened display IDs.
Hand off its file,
digest and complete refs list. A code proposal generally includes more than
one ref; repeat all of them on `receive`. The receiver inspects the exact code
and submission before manual admission:

```sh
hn contribution pending --json
hn contribution inspect SUBMISSION_ID --json
hn contribution decide admit --submission SUBMISSION_ID --expect-heads absent --operation admit-001 --json
hn proposal show PROPOSAL_ID
hn proposal status PROPOSAL_ID
```

`admit` only admits that revision. The normal base-policy review, trusted CI
results, maintainer acceptance and merge requirements still apply. A received
actor or receipt cannot install reviewers, runners, policy or keys. No receipt,
admission or query executes submitted code. Explicit `hn run request PROPOSAL_ID
PIPELINE` and `hn run execute REQUEST_ID` retain native authority/evidence and
isolation checks. Host execution requires the explicit
`--backend host --allow-unsafe-host-execution` opt-in on that invocation.
Under enabled contributions, the first profile refuses the `hn runner` family
and `hn run list`; use explicit request/execute and raw `hn log` or exact raw
inspection. Enabling contributions does not enable a worker.

## Moderate without rewriting history

Only a currently evaluated project moderator can sign these decisions. Inspect
the exact target and all current decision heads first:

```sh
hn contribution decide reject --submission SUBMISSION_ID --expect-heads absent --operation reject-001 --json
hn contribution decide hide --revision REVISION_ID --expect-heads absent --operation hide-001 --json
hn contribution decide restore --revision REVISION_ID --expect-heads DECISION_ID --operation restore-001 --json
hn contribution decide block --actor ACTOR --expect-heads absent --operation block-001 --json
hn contribution decide unblock --actor ACTOR --expect-heads DECISION_ID --operation unblock-001 --json
hn contribution pending --json
hn contribution inspect --raw REVISION_ID --json
hn log
```

`absent` means no prior decision head for that exact typed target. For concurrent
heads, use their complete comma-separated IDs as `--expect-heads ID1,ID2`; an
authorized successor joins them. Conflicts remain inspectable and fail closed
for the affected target rather than silently choosing the last arrival. Other
valid work remains inspectable. Exact operation replay returns its original
decision; reusing an operation name with changed intent refuses.

Hide/reject/block affect this project's governed view. Signed raw payloads,
signatures and retained refs remain immutable and explicitly inspectable. A
hidden admitted revision remains valid lineage, but cannot authorize new hidden
references. Hiding or draining content does not refund storage. No command
promises global erasure from other replicas or a storage ACL against someone
who can write directly to the host/Git repository.

Moderator removal/revocation is a native policy amendment. Exact already
established receipt/decision checkpoints (`contributionCheckpoints`) must be
ratified in native governance when preserving earlier authority across changed
policy digests. Copying checkpoint JSON and Git ancestry alone supplies no
ratification. `revokedModerators` makes the named actor's historical decisions
ineffective in the current view and reports them as revoked. Old credentials
cannot manufacture established history under a prior policy. See the detailed
contract before constructing a checkpoint amendment.

Disable through the same governed policy path by removing `contributions`.
Legacy native views then apply; raw contribution facts and local accounting
remain. Re-enabling the same scope preserves charges and authentic established
history; raw imports during the disabled period do not become automatic intake.

## Retry, recover and account for capacity

Record a durable receiver-local `--operation` before intake. An exact retry with
the same digest and ref selection reuses the original durable attempt without
reopening a changed or missing source file or spending another reservation.
Changing digest/selection under that operation refuses. A failed, reserved or
uncertain attempt is not proof of admission, even if a retry exits successfully:

```sh
hn contribution intake --operation inbox-001 --json
# Retry the exact original receive command and inspect status again.
hn contribution pending --json
```

Intake inspection reports measured wire/decoded bytes, objects/events, status,
reservation and receiver-local ledger. Preserve the journal and retained files
after interruption; do not delete them to retry or infer that missing progress
means no effects. Exact public retry reports a retained `reserved`, `failed`
or `uncertain` state without promoting it. There is currently no public
`hn contribution recover` command. Deterministic durable-artifact reconciliation
is covered at the source adapter; public operator recovery for such an
interrupted attempt remains a limitation requiring inspection/support. Native
operation retries can finish their retained exact local transaction; inspect
their outcome rather than generating a different operation blindly.

The following bounds apply before supported public-file promotion:

| Boundary | Guarantee and limit |
| --- | --- |
| Source | Already local regular file, no symlink/FIFO/device/URL/network fetch; reservation precedes open. Oversized files refuse before content read. |
| Per request | Positive finite wire, decoded byte, Git object, event, dependency depth and deadline bounds; complete original actor ancestry must fit. |
| Shared project | One receiver-local window and aggregate retained budget across keys, automatic admissions, pending/admitted/hidden content and linked worktrees. |
| Failure | Failed/canceled/uncertain reservations stay charged; exhaustion refuses before new input I/O and adds no unbounded refusal journal. |
| Cancellation | Public adapter checks context during bounded reads/subprocess work. Trusted local legacy native validation checks at boundaries; it does not interrupt every synchronous internal Git call. |

Reservations conservatively charge the configured wire maximum plus decoded
maximum plus 4 KiB accounting metadata, and object/event maxima. With the listed
defaults, each reservation costs about 48 MiB: a 256 MiB window fits five;
1 GiB retained fits 21 before other bounds/operations. Public automatic receipt
publication can require a second control reservation. A `requests: 256` cap
therefore does **not** promise 256 successful messages. The acceptance fixture
uses smaller 128 KiB wire/1 MiB decoded/1,024 object/256 event limits; these are
test choices, not an established practical workload recommendation.

Only a current maintainer deliberately rolls the window:

```sh
hn contribution rollover --expect initial --window reviewed-next-window
```

Rollover preserves aggregate retention charges. It cannot reuse old windows or
repeatedly roll empty windows to acquire capacity. Restart, queue draining,
moderation and new keys never reset charges. The current CLI has no general
garbage-collection command that promises refunds for retained immutable history.
Any budget amendment must use native governance; preserve failed/uncertain
history. This ledger is local to the receiver, not a global distributed quota.

Ordinary Git fetch/sync may receive a pack before measuring it. Their subsequent
refusal is **not** a hard pre-download wire cap. The supported public-file path
does not promise bounds on an earlier file copy performed outside `hn`.

## Reconstruct on a keyless observer

Keep code plus the **exact selected** native actor/proposal/code and contribution
refs and referenced objects. Ordinary clone omits custom refs. Ordinary
`hn sync` authenticates selected native transport; it does not copy contribution
refs or adopt authority merely because Git could transfer them.

For eligible outer histories, an explicit artifact is convenient:

```sh
# Existing selected project:
hn contribution archive --output ./contributions.hnc --json
# Fresh clone with independently obtained code/policy, no hn init:
hn contribution restore --file /path/to/contributions.hnc --digest FILE_DIGEST --operation observer-restore --ref EXACT_RETURNED_REF --json
# Repeat --ref for every archive ref; then explicitly inspect/adopt authority:
hn contribution authority set --ref refs/heads/main --expect absent --read-only --json
hn contribution inspect --json
hn identity show --json
```

The final identity command should report no local signer. Restore alone adopts
no authority or key and publishes no native facts before explicit selection.
Read-only adoption/retained registration charges this receiver's verified
coverage; portable receipts do not supply local storage allowance.

The outer `hn.contribution/1` profile refuses native review/run/decision/merge/
audit/memory predecessors. It never truncates actor ancestry, and independently
prefetched facts cannot repair an unsupported outer chain. Native ratification
history must be transported separately using explicit native replication
selections. Mixed trusted backups can instead use `git bundle create` with an
operator-reviewed exact list of code, native and `refs/hn/contributions/...`
refs, followed by explicit same-ref fetches on recovery. Retain their OIDs and
compare them after restoration. Do not label an outer artifact a universal
backup of mixed histories or copy private identity directories into it. See
[recovery.md](recovery.md) for the separate public/private recovery tracks.

If direct Git additions arrive after adoption, governed reads refuse unregistered
coverage instead of silently spending capacity or selecting authority:

```sh
hn contribution retained show --json
hn contribution retained register --expect SNAPSHOT_DIGEST --json
```

Review the returned snapshot digest before registration. Registration remains
bounded and charged; an exhausted receiver cannot use imported signed receipts
to reset its allowance. Never restore private keys for a normal observer.

## Evidence and remaining qualification

`TestBetaContributionPublicVariants` in
`beta_contribution_acceptance_test.go` runs standard and actual combined
`nogithub,noforge` source executables with a closed, Git-only child environment.
Its local configured upstream rejects a valid push; a writable control accepts
the same source/ref. The journeys cover stranger files, automatic issue/reply
admission, manual proposals, native evidence/execution refusals, keyless recovery,
scoped moderation, many-key saturation, malformed input and exact retries.
`TestBetaContributionMeasuredInterruptionAndCancellation` supplies separately
labeled source-adapter read/cancellation/interruption measurements.

The same public journeys accept explicit `--hn-package-dir` and
`--hn-package-version` selections through the existing package verifier. Package
mode requires matching BUILD provenance, current commit/target/variant/version
and executable digest and never silently builds source on refusal. A source
test pass is not a package, external adopter, provider or platform qualification.
These local reproducible journeys supply pilot/boundary-review material;
independent user findings, workload limits and final beta qualification remain
the later mission gates. Missing observations remain pending.
