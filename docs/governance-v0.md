# Governance v0

Hubnot policy answers which verified signed claims count for a project. It is
a normal versioned file at `.hn/policy.json`:

The active policy starts with fresh `hn` actor fingerprints. Roles are never
imported from `.nh/policy.json`, `.git/nh/**`, or `refs/nh/*`; those paths can
remain as historical evidence but provide no authority to this protocol.

```json
{
  "version": "hn.policy/0",
  "maintainers": ["<full-actor-fingerprint>"],
  "proposals": {
    "requiredApprovals": 1,
    "requiredAccepts": 1,
    "trustedReviewers": ["<full-actor-fingerprint>"],
    "allowAuthorApproval": false
  },
  "pipelines": {
    "test": {
      "requiredResults": 1,
      "trustedRunners": ["<full-actor-fingerprint>"]
    }
  }
}
```

A pipeline may also list `requesters`: 1–64 actors, besides maintainers,
whose run requests count for that pipeline. Omit the field to keep run
requests maintainer-only. Requesting a run grants nothing else: results still
count only from `trustedRunners`, and requesters gain no review, decision or
merge authority. Older clients reject policies that contain the field.

```json
"test": {
  "requiredResults": 1,
  "trustedRunners": ["<runner-actor>"],
  "requesters": ["<orchestrator-actor>"]
}
```

Actor values are complete lowercase SHA-256 fingerprints of Ed25519 public
keys. Unknown JSON fields, duplicate or malformed actors, an empty maintainer
list, unsatisfiable thresholds, invalid pipeline names, and unsupported policy
versions are rejected. Policy files are limited to 1 MiB.

## Inspect policy

```sh
hn policy show [REV]
```

The revision defaults to `HEAD` and resolves to one full commit object ID. The
command runs the canonical parser and validator and prints:

- the resolved commit and full `sha256:<64-hex>` digest of the exact policy
  bytes;
- every maintainer and the required acceptance count;
- every trusted reviewer, approval threshold, and author-approval rule;
- pipelines in lexical order, with required results and full trusted-runner
  actor IDs.

Trust-bearing actor IDs are never shortened in this output.

Trust-bearing commands also require complete `sha256:<64-hex>` event IDs.
Prefixes are display conveniences and are rejected before event lookup, even
when a local prefix would happen to be unambiguous.

## Validate an amendment

```sh
hn policy check --base REV --head REV
hn policy check --base REV --file PATH
```

Exactly one of `--head` and `--file` is required. Both the base and proposed
policy go through the same canonical validation used during candidate
evaluation. `--file` reads an explicit working-tree draft and does not mutate
it. The command prints resolved commit IDs when applicable, both full policy
digests, and a deterministic comparison of:

- exact-byte change;
- maintainers and required acceptances;
- trusted reviewers, required approvals, and author approval;
- added and removed pipelines;
- required-result and trusted-runner changes for every pipeline.

Pipeline names and actor IDs are sorted. A malformed side is identified as
`base` or `proposed`, and validation fails before a candidate is prepared.

## An amendment is an ordinary candidate

A policy amendment introduces no administrator event or mutable control plane.
It is a normal immutable proposal candidate whose `head` changes
`.hn/policy.json`:

```sh
hn policy check --base main --file .hn/policy.json
git add .hn/policy.json
git commit -m "amend collaboration policy"
hn policy check --base main --head HEAD
hn proposal open --base main --head HEAD \
  --body "Governed only by the signed base policy." \
  "Amend collaboration policy"
```

When valid policy bytes differ, `hn proposal open` prints both full digests and
states which base digest governs. The proposed policy becomes relevant only to
later candidates whose signed base commit contains those bytes. A candidate
cannot add a permissive actor or lower a threshold and then use that proposed
authority to accept itself.

## Immutable evaluation context

A candidate signs both a base and a head commit. Its governance policy is
always loaded from:

```text
<signed-base-commit>:.hn/policy.json
```

The SHA-256 protocol ID of those exact bytes is signed by decisions and merge
facts. Policy authority comes only from the explicit actor fingerprints in
that base document.

Identity-continuity projection and replication selection are deliberately
non-authoritative inputs:

- an accepted device/successor relation may explain an actor's relationship,
  but cannot transfer or inherit a role;
- a selected and cryptographically valid actor history is visible, but its
  evidence counts only if that exact actor appears in the applicable base
  policy;
- a successor, related device, or similarly named actor is never substituted
  for a listed actor.

## Readiness and exact evidence

`hn proposal status <full-candidate-id>` independently derives readiness from
accepted repository data:

1. the candidate code ref exists and equals the signed head;
2. the configured number of distinct trusted reviewers currently approve;
3. author approval counts only when the base policy permits it;
4. every required pipeline has enough distinct passing trusted runners;
5. each run request was signed by a base-policy maintainer, or by an actor in
   that pipeline's optional `requesters` list, and binds the exact candidate,
   head, pipeline name, and pipeline-definition digest.

A reviewer's latest review is current. Change requests are displayed but are
not a protocol-level veto. A runner's latest exact result is current for a
pipeline. Evidence for a predecessor or sibling candidate never qualifies a
revision, even when code or lineage is related.

Before publishing an acceptance, the client reloads the verified event set and
re-evaluates the exact base policy. An accept decision signs the exact review
and passing run-result IDs selected as evidence. A rejection requires an
explanation and does not require readiness.

Only actors listed as base-policy maintainers can publish qualifying decisions.
A maintainer's latest decision is current; a current rejection blocks merge.
The required number of distinct current acceptances must be present.

## Merge and conflict recovery

`hn merge <full-candidate-id>` requires:

- a clean worktree on a named branch;
- a current branch descended from the signed base;
- an accepted candidate ref matching the signed head;
- enough base-policy acceptance decisions and no current rejection;
- the active actor to be a base-policy maintainer.

Immediately before changing Git state, merge reloads facts, policy, evidence,
and lineage gates. Git performs a `--no-ff` merge. A successful merge emits a
signed `proposal.merged` event binding the candidate, proposed head, resulting
merge commit, exact base-policy digest, and acceptance decision IDs.

The merge commit and merge event exist locally before either publication step.
Publish collaboration refs with `hn sync`; publish the primary branch with an
ordinary explicit `git push`. Either action can be retried without rewriting
the signed fact.

If Git created the merge commit but event creation or append failed, rerun
`hn merge <full-candidate-id>`. When the proposal head is already contained,
the repair path reloads and verifies current facts, policy, readiness,
acceptance evidence, maintainer authority, and lineage. It records the missing
event only when exactly one first-parent merge commit since the signed base
directly names the proposal head as a parent. It refuses stale evidence,
ambiguous or non-direct containment, an existing merge fact, and shallow data
until the exact dependencies have been recovered. It never performs another
code merge during repair.

On a Git conflict, Hubnot aborts the merge and restores the previously clean
worktree. Resolve the code with Git and publish a new immutable
`proposal.revise` candidate with explicit base and head commits. The original,
siblings, and their evidence remain inspectable. The revision requires new
exact review, CI, and acceptance evidence.

There is no timestamp or delivery-order winner among siblings. A candidate
with successors is superseded; a lineage with a merged member is closed; and
multiple valid merge facts produce a visible merge conflict rather than an
invented winner.

## Optional check protection and automerge

Check protection and automerge are independent repository opt-ins. Omit either
section from `.hn/policy.json` to keep it disabled; `null` also disables it in
clients that support these fields. An empty or incomplete object is invalid.
Initialization, upgrades and imports do not install rules or enable automation.
Enabling either capability is an ordinary governed policy amendment.

The following complete example enables both. Replace every `<...-actor>` with a
full lowercase 64-hex actor fingerprint from `hn identity public`; these are
placeholders, not actor names or provider accounts. The required `lint` pipeline
must exist at `.hn/pipelines/lint.json`.

```json
{
  "version": "hn.policy/0",
  "maintainers": ["<maintainer-actor>"],
  "proposals": {
    "requiredApprovals": 1,
    "requiredAccepts": 1,
    "trustedReviewers": ["<reviewer-actor>"],
    "allowAuthorApproval": false
  },
  "pipelines": {
    "lint": {
      "requiredResults": 1,
      "trustedRunners": ["<runner-actor>"]
    }
  },
  "protectedChecks": {
    "paths": [".golangci.yml", "scripts/lint/"],
    "trustedReviewers": ["<check-reviewer-actor>"],
    "requiredApprovals": 1
  },
  "automerge": {
    "rules": [
      {
        "name": "docs-only",
        "targetBranch": "main",
        "actors": ["<maintainer-actor>"],
        "authors": ["<author-actor>"],
        "paths": ["docs/"]
      }
    ]
  }
}
```

For protection alone, omit `automerge`. For automerge alone, omit
`protectedChecks`. A rule never creates a missing review, passing run, or
acceptance, and it never lowers the existing thresholds.

Inspect the configuration and proposed amendment before committing it:

```sh
hn policy show main
hn policy check --base main --file .hn/policy.json
```

Inspection and comparison expose activation, protected paths, designated
reviewers, thresholds and every automerge rule's actors, authors, paths and target.
Older clients that do not recognize these fields reject policies containing
them, including a `null` field. Omit the fields for compatibility with such
clients. Existing signed bytes and historical facts are not rewritten.

### Protected paths and exact review

While `protectedChecks` is enabled, these paths require designated approval:

- Every configured literal file or directory.
- `.hn/policy.json`, including changes that remove or disable protection.
- `.hn/pipelines/<name>.json` for every pipeline required by the signed base policy.

A path is repository-relative. A directory selector ends in `/` and covers its
descendants. Selectors have no glob or expression syntax; absolute paths,
traversal, noncanonical paths, control characters, backslashes, wildcard
characters, colons and `.git` components are rejected. Configuration permits at
most 64 selectors, each at most 4096 bytes. `paths: []` is allowed for protection;
the policy and required pipelines are still protected.
Changed-path Git output is limited to 8 MiB; exceeding the bound refuses the
operation rather than evaluating a truncated change list.

Protection compares Git trees, not the working copy. Additions, removals,
renames, content changes, mode/type changes and replacement of an ancestor count.
A required pipeline cannot evade review by moving or replacing its definition.
Select lint configurations, invoked scripts and other check inputs deliberately.
Protection gates changes to those files; it does not determine whether every
possible source-code or linter change is semantically weaker.

Designated approvals are ordinary `hn review C --approve` events for that exact
candidate. They count only from distinct listed actors other than the candidate
author, even when ordinary policy allows author approval. A designated actor
need not be an ordinary trusted reviewer. An approval may satisfy both roles
only when that exact actor is listed in both places; both thresholds still apply.
There may be 1–64 designated reviewers, and the required approval count must be
between 1 and that reviewer count.

`hn proposal status C` reports changed protected paths and current designated
approval counts. An acceptance signs the exact qualifying review IDs alongside
its ordinary review and run evidence; local and fetched evidence admission
validate those bindings. Revisions need new exact-candidate evidence. A later
withdrawal blocks a new protected merge without invalidating what an earlier
decision originally signed.

### Signed-base status and the current target

Status and immutable acceptance evidence are evaluated against the candidate's
signed base. A new manual merge also checks the actual target. If protection
is enabled there, revise against the current target when either:

- Its exact policy digest differs from the candidate's base-policy digest.
- Any protected path changed from the signed base to the current target,
  even when policy bytes stayed identical.

This prevents a candidate from incorporating a target's stricter lint check,
restoring the weaker old base bytes, and hiding that restoration in an otherwise
documentation-only base-to-head diff. An older acceptance is not authority to
ignore the target's protected baseline. Unprotected target changes alone do not
impose this additional revision requirement. The existing contained-merge repair
path remains governed by the original signed base and evidence.

### Rule-based native automerge

An automerge policy contains 1–64 uniquely named rules. Every rule explicitly
names a local target branch, 1–64 executing actors, 1–64 authors and 1–64 literal
allowed paths. Executing actors must also be base-policy maintainers. Directory
allowlists permit descendants only; allowing `docs/` does not permit replacing
the `docs` ancestor itself. Every changed path must match a rule selector.

```sh
hn proposal status <full-candidate-event-id>
hn automerge <full-candidate-event-id> --rule docs-only
hn automerge <full-candidate-event-id> --rule docs-only --execute
```

The default command is a read-only, keyless preview. It prints full candidate,
base, head and policy identities, the observed target, allowed executing actors,
changed paths and blockers. A blocked preview returns failure. An eligible
preview establishes candidate/target eligibility; execution additionally checks
the selected private signing identity. A preview result never authorizes a later
execution without reevaluation.

Execution requires current ordinary and designated approvals, required passing
pipeline results, and the existing maintainer acceptances. It also refuses current
trusted change requests or pipeline failures, maintainer rejections, superseded
or closed lineages, mismatched code refs, disallowed authors or changed paths,
policy amendments, and protected-check changes. The latter two take the manual
path even after they receive the required review and acceptance.

The checked-out branch must match the named rule, be clean, and still equal the
candidate's exact signed base. After target movement, create a revision and obtain
fresh evidence. Unfinished merges, rebases, cherry-picks, reverts, sequencer
operations or bisects refuse automation until finished or aborted.

Automerge also refuses shallow repositories and active partial/promisor
configuration before object evaluation. Use a full clone, or explicitly hydrate
and reconcile its configuration outside this command. Downloading some missing
objects alone does not make a configured partial clone eligible. Selected data
that suffices for an ordinary manual merge does not waive this full-repository
requirement. Automerge does not fetch or invoke shallow recovery.

### Commit path, publication and recovery

Automatic execution creates a native two-parent merge commit from the verified
head tree and uses an expected-old Git ref transaction for the actual target
advance. It rechecks current evidence and target while holding that lease, then
records the existing signed `proposal.merged` fact with the exact acceptances.
Ignored files that the checkout would overwrite cause refusal.

This commit path uses `git commit-tree`, not ordinary `git merge`/`git commit`.
It does not run `pre-merge-commit` or `pre-commit` hooks. Put required checks in
native pipelines and configure their trusted runners in policy; a local Git
merge hook is not an automerge acceptance gate. Operator-controlled Git
`reference-transaction` hooks retain their normal behavior.

No command here starts a daemon, executes a pipeline, creates an approval or
acceptance, synchronizes, or pushes. Publication remains explicit:

```sh
hn sync origin

git push origin main:main
```

If code merged but its signed fact could not be recorded, inspect the target
and retry `hn merge C` for the existing manual repair. Repair requires current
qualifying evidence and one unique first-parent merge that directly names C's
head; it adds only the missing event. Repeating automerge does not create a
second merge when the target already moved. A failure before ref advancement
can leave unreferenced Git objects; an interrupted checkout can leave staged
candidate changes on the old target. Preserve local work, inspect `git status`
and the target, and reconcile the checkout before retrying. Do not discard local
changes merely to make a retry eligible.

These gates enforce Hubnot's native merge path. They do not prevent a process
with direct host or Git write access from changing branches outside Hubnot, and
the local ref lease is not a distributed transaction across arbitrary writers
or remote replicas.
