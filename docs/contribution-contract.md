# Native contribution foundation (hn.contribution/1)

WP10 specifies and implements a library contract. The commands below are the
WP11–13 integration contract, **not commands implemented by WP10**. Existing
native commands keep their behavior until live integration is delivered. No
endpoint, daemon, provider account, second identity or automatic worker is added.
Contributions, protected checks, automerge and audits are independent opt-ins.

## Project and authority

The optional `.hn/policy.json` member `contributions` is omitted when disabled.
When present it contains `project` (a stable `sha256:` identifier), `moderators`
(full existing actor fingerprints), `rules`, `limits` and `baseline` (a sorted,
finite list of exact native revision IDs). A project creator chooses a random
32-byte scope once; its identifier, retained in authorized policy history, is
never a mutable branch OID, remote URL, filesystem name or provider identity.
Forks adopting that scope deliberately join that project's moderation domain;
an independent fork chooses another. A local authority anchor selects a direct
`refs/heads/NAME` with compare-and-swap (`absent` or exact prior anchor digest),
following the existing `AuditAuthoritySet` selection pattern. It remains a
separate contribution selection, so audits need not be enabled. A received
archive, key, role list, remote branch or source policy never installs an anchor.

Bootstrap is an explicit local operator trust decision: inspect the already
present branch and current policy; activation/enabling requires an active
signer who is a maintainer of that policy. A keyless observer may explicitly select the same branch for
read-only trust and reconstruction; that selection cannot activate policy, sign
a decision or change authority. Possessing an unknown key is insufficient.
Once adopted, policy changes follow the existing base-policy proposal decision
and merge path; proposed maintainers cannot approve their own installation.
Authority advancement must be a verified descendant of the adopted branch
history, and non-fast-forward replacement requires explicit anchor readoption.
Keyless observers explicitly select the same authority to rebuild the governed
view; raw verification remains possible without adopting it. This cannot
cryptographically prove the initial human/operator selected the intended project.

Each decision records exact policy commit and digest and exact prior decision
heads. The host resolves these from selected authority ancestry and checks the
signer's historical role. Current policy controls new operations and explicitly
revoked moderator actors. Ordinary policy updates can preserve previously legitimate
decisions through an exact established-history checkpoint: before adopting a
changed policy, existing base-policy governance approves a finite, sorted list
of exact already-received receipt/decision IDs as committed project checkpoint
data. The host `AuthoritySource.Established` verifies the exact ID and evaluated
policy against this checkpoint in **current selected native governance history**.
An envelope field, old moderator signature, local timestamp or foreign receipt
is never establishment. Checkpoint lookup/closure is bounded by project object,
event, depth and deadline limits and the static 16,384-record graph maximum;
excess history refuses rather than inferring authority. Every authority-bearing
record with a different policy digest requires this proof. Undelivered old
decisions are unestablished and need fresh current authorization. Live writers
use the current policy digest and recheck it before signing/promotion. Removing a
moderator therefore prevents newly manufactured old-policy decisions; explicit
`revokedModerators` invalidates that
actor's historical decisions in the current projection (visible as revoked), and
removing that revocation restores their historical effect. This supplies reversible
project moderation without rewriting history or permanent policy-SHA freezing.
Disable removes the optional member through the same authorized policy path.
While disabled, legacy views apply and contribution facts/accounting remain raw
history. Re-enabling the same scope reuses receipts, baseline and retained charges;
it never treats intervening raw imports as admitted. Baseline enlargement is an
explicit bounded authority amendment with charged exact receipts, never a wildcard
"everything before now" timestamp or raw-head shortcut.

## Concrete local journey and wire

These are planned CLI spellings for downstream implementation:

```sh
# Maintainer: after legitimate policy adoption/merge, select local authority.
hn contribution authority set --ref refs/heads/main --expect absent
hn contribution authority show --json
# Stranger: hn init supplies the existing Ed25519 key; no upstream push needed.
# Prepare native facts locally using existing issue/discussion/proposal writers.
hn contribution prepare --revision sha256:REV --output ./issue.hnc
# Carry this regular local file by ordinary out-of-band means.
hn contribution receive --file ./issue.hnc --digest sha256:FILE --operation inbox-001 --json
hn contribution pending --json
hn contribution inspect sha256:SUBMISSION --json
hn contribution decide admit --submission sha256:SUBMISSION --expect-heads absent
hn contribution decide hide --revision sha256:REV --expect-heads absent
hn contribution decide restore --revision sha256:REV --expect-heads sha256:HIDE
hn contribution decide block --actor FULL_FINGERPRINT --expect-heads absent
hn contribution inspect --raw sha256:REV --json
```

`sha256:REV` abbreviates a full 64-hex digest; abbreviation is not valid wire
input. The first transport accepts only already local regular files, opened
without following symlinks after reserving declared worst-case limits. FIFOs,
devices, URLs, network refs and remote Git fetch are refused. Context deadlines
are checked between bounded local reads; this does not promise cancellation of
arbitrary blocking readers or network I/O. Complete native actor/dependency
closure must fit the limits; long histories are refused honestly. No arbitrary
ref update, code checkout, pipeline execution or upstream push follows receipt.

A contribution file contains bounded manifest, exact signed envelope(s), and a
native `hn.project-archive/1` with exact native facts and Git object closure.
WP11 defines its container framing and uses `migration.ReadArchive` followed by
`StageProjectSnapshot` for native verification. An envelope is **not a native
fact verifier**: its own signature binds the request to those independently
verified native bytes. Detached envelope payload/signature are stored as blobs
in manifest trees
under separate **commit** refs `refs/hn/contributions/PROJECT/ACTOR`. Do not
inject new kinds into legacy actor chains. Existing native snapshot validation
accepts only actor/proposal/memory/code commit refs and cannot validate these
new manifests. WP11 must implement an outer artifact adapter that packages the
envelopes plus the existing native archive; WP12 must explicitly validate those
commit manifests during contribution reconstruction and full backup/import.
Until those adapters exist, ordinary native sync/migrate cannot be claimed to
transport these envelopes. A complete backup includes the outer contribution
artifact, its native archive, all retained contribution manifest commit refs,
selected-authority checkpoint/policy history and local intake ledger separately.
The public outer artifact excludes private keys and local authority selection;
a keyless observer explicitly selects verified authority after import.
The first supported backup/restore journey is explicitly the outer artifact:

```sh
hn contribution archive --output ./project-contributions.hnc
# Fresh keyless repository, with independently obtained trusted branch history:
hn contribution restore --file ./project-contributions.hnc --digest sha256:FILE --operation restore-001
hn contribution authority set --ref refs/heads/main --expect absent --read-only
hn contribution inspect --json
```

Archive includes the complete selected contribution graph and its inner native
closure within finite project bounds; oversized full-history backups refuse.
Restore reserves receiver-local capacity before opening the file, authenticates
raw records without adopting authority or signing receipts, and retains original
project receipts for observer reconstruction after explicit read-only selection.
Local reservations are never replaced with the producer's accounting. Portable
checkpoint data travels as ordinary code/authority commits in the inner archive;
the host checks its exact project and base-policy-governed acceptance. WP12 can
implement that checkpoint adapter in its assigned `policy.go`,
`contribution_commands.go` and `contribution_records.go`; WP11 outer exchange
stays in `contribution_exchange.go`. General `hn sync` and legacy migration
archive inclusion are not part of this initial transport contract. If later
integration needs `project_snapshot.go` or native migration schema changes,
canonical ownership must be amended before those changes; unsupported contribution
refs must never be disguised as an existing native snapshot kind.

A record ID is SHA-256 of its exact JSON payload, signature is raw Ed25519 over
those bytes, actor is the existing lowercase SHA-256 fingerprint of the public
key, and public key uses unpadded standard base64. Canonical encoding is required:
unknown/duplicate/case-variant fields, trailing bytes, noncanonical JSON, invalid
UTF-8, excessive depth and nonpositive limits are rejected before acceptance.
The static schema and signature never depend on today's mutable policy.

Illustrative payload (full digests substituted at runtime):

```json
{"version":"hn.contribution/1","kind":"submission","project":"sha256:PROJECT","actor":"ACTOR","publicKey":"BASE64","intent":"issue-001","submission":{"operation":"issue.create","content":{"root":"sha256:REV","revision":"sha256:REV","author":"ACTOR","parents":[],"references":[]}}}
```

Decision payload uses `kind:decision`, `decision:{action,target,authority,parents}`;
`target` is either `{kind:submission,id:SUBMISSION}`, `{kind:revision,id:REV}` or
`{kind:actor,id:ACTOR}`. `authority` is `{commit:GIT_OID,digest:POLICY_DIGEST}`.
A portable `kind:receipt` uses `receipt:{submission,authority,artifact,accounting,
window,cost,automatic,evaluated}`. It binds an exact submission ID (and thus
original content), exact file digest, measured/reserved request cost, receiving
window, durable local accounting-transition digest and the sorted exact actor/
revision moderation decision heads evaluated at intake. Its signer must be a
maintainer or moderator under evaluated project authority, with current-policy
or established-history proof. The signature makes automatic admission portable;
the local reservation journal alone cannot authorize a shared view. Both manual
admit and automatic receipt require completed bounded-intake provenance. An
observer verifies these signed attestations without acquiring the receiver's
private ledger; another receiver must still measure and reserve its own local
retention, and never uses the foreign receipt to spend local capacity.
Admit/reject targets a submission; hide/restore an exact revision; block/unblock
an actor in this project. Every record has bounded nonempty actor-local `intent`.
The same actor/project/intent and exact signed bytes replay once. Different
payloads with that intent are visible conflicts and cannot select a winner by
arrival order or lexicographic ID. Envelopes reuse existing identity keys; a
valid signature proves control of a key, not one person or spam-free content.

## Typed policy, intake and projection

`Evaluate` distinguishes transport verification, submission admission, author
edits/replies and moderation from native code review, acceptance, execution and
merge. Transport authenticity grants none of the latter. Explicit rules may
allow automatic issue creation and replies by unknown keys; automatic admission
is never a read-time inference. It runs only during bounded intake and writes an
exact durable receipt before promotion. Manual admit/reject uses a current
moderator; an author edit/revision must bind the original author and admitted
parent revisions unless an explicit maintainer rule permits that operation.
All unsupported operations fail closed. Review/run/accept/merge return a distinct
"existing authority required" result and retain all original native guards.

The signed receipt explicitly attests that its `evaluated` list is the complete
receiver-known relevant moderation-head set and that all exact parents/references
were admitted and visible at intake. `IntakeEvidence` computes this cut and refuses
hidden dependencies or blocked actors; the host must call it and compare the cut
and current authority under its intake lock immediately before signing. The
reducer rejects contradictory listed evidence, including a historical hide/block
whose signer was subsequently revoked. It preserves legitimate earlier edits
when a parent is hidden later. A signature by a trusted project intake signer is
an attestation of this local observation, not proof of global absence or a total
order across disconnected replicas: offline observers cannot detect a trusted
signer lying by omitting a known decision. This is the same explicit project
signer trust required by the durable-accounting attestation. WP12/13 must exercise
the live hidden-parent refusal through this shared boundary.

The library consumes native-authenticated typed facts through `NativeSource` and
policy snapshots through `AuthoritySource`. These are trusted host adapters,
not values supplied by a contribution file. They reuse `verifyEvent`, native
issue/resource relationship validation, proposal lineage and existing exact
policy source verification. Host adapters must classify operations and author,
root, parents and typed references from signed native facts; the envelope must
match them exactly. Raw fact storage is policy-neutral. Shared `Reduce` computes
admitted revisions before native catalogs/lineage are built. It returns both
admission and visibility plus decision conflicts/revocations; hidden revisions
remain admissible lineage facts but cannot authorize a new reply/edit reference.
Rejected/unadmitted revisions never become operative heads. A hidden parent is
an explicit tombstone in the governed view, never a dangling body leak. Replies
with a hidden required reference are suppressed until that reference is restored.
Restoring a hidden predecessor does not resurrect an inadmissible successor.

`Content.References` is a sorted set of `{kind:"root"|"revision",id:ID}`,
classified from the actual signed native field. Root identity references require
all governed current state heads to be visible; they do not pin the opening
body. Exact revision references require that particular revision to be visible,
including when it is the original opening ID. In `issue_projection.go`, comments
are separate from the state-revision heads: classify an immutable `issue.comment`
as its own `Content.Root == Revision`, `issue.reply` operation and a **root**
reference to `Subject`, rather than blindly using `issueRootID`. Issue revisions
consume their explicit state parents; comments never consume a state head.
`resource_engine.go` similarly gives each discussion-reply.open its own root.
DiscussionID, ReplyTo, ProjectID and issue relation targets are root references;
AcceptedReplyRevision and ProjectRevision are exact revision pins. Revision
Subject/self-root and Parents remain lineage dependencies, not dynamic visibility
references to the resource's own current head. A proposal reference preserves the
exact proposal revision named by the native field. The host classifier reuses
native relationship verification before emitting these typed facts.

`View.Roots` and `View.Heads` describe authenticated revision lineages after
admission; `IntakeEvidence` checks an edit's exact parents and current resource
heads, and a root-reference reply's current target heads. This prevents an old
visible opening from authorizing a reply to a hidden newer state. Hide of a
comment alone cannot hide its issue. Visibility uses a greatest fixed point with
shared root gates: an accepted discussion answer can refer to a reply that
references the discussion root without inventing a native dependency cycle.
A hidden exact reference propagates through that derived visibility cycle; valid
unhidden cycles remain visible. Raw authentication/dependency depth still follows
the actual native DAG and is independent of these derived root aliases.

Decision graphs are per exact typed target and project. Parent IDs must name
same-target, historically authorized decisions; every available current head
must be named by a live writer. Concurrent heads stay visible and fail closed
for that target (no admit, no visibility, blocked actor) until an authorized
successor joins them. Revoked/invalid decisions cannot erase valid heads;
structural errors isolate the offending record and dependent nodes with
explicit diagnostics before promotion. They cannot make unrelated pending work
or governed content uninspectable; invalid descendants cannot erase valid heads.
Submission parents/references must be admitted; raw actor-chain dependencies are
still retained/authenticated and charged even when they have no governed role.
The finite baseline lists only exact verified native revisions and dependency
closure, checked independently of current moderation effects; local baseline reservation receipts are charged and durable before
enable completes. The authenticated adopted baseline policy supplies portable
project admission for this explicitly finite historical set; no old author must
re-sign historical facts or manufacture a new submission.

Before opening or staging input, WP11 must reserve one project operation budget
under a lock and durable journal. This is a **receiver-local per-project**
ledger, not a distributed global quota. Imported receipts, remote window names
and actor keys cannot supply or reset local accounting. Limits are positive
finite per-request wire,
decoded bytes, objects, events, dependency depth and milliseconds plus project
window bytes/objects/events/requests and aggregate retained bytes/objects/events.
The window is an explicit authority-selected identifier, never clock inference.
A reservation includes a fixed 4 KiB retained accounting-metadata charge in
addition to wire and decoded maxima; receipt/journal records and retained failed,
canceled and uncertain attempts have finite count/byte capacity. Once exhausted,
a refusal writes no additional unbounded journal entry.
Every intake attempt that proceeds to I/O reserves, including malformed/canceled
input; exact replay reuses
its durable receipt, conflicting intent refuses. Charge automatic admissions,
metadata, predecessor closure and different actor keys to the same project.
Pending→admitted/rejected/hidden, draining, disabling and restart never reset
charges. Window rollover is a deliberate current-maintainer action recorded with
expected old window; aggregate retained charges survive rollover. An empty
window cannot roll over repeatedly: each consumed window transition uses the
bounded accounting metadata already reserved by at least one request. Prior
window names cannot be reused. Exact replay performs no new I/O and can inspect
a prior receipt even after policy lowers limits. Reclamation
can release only bytes/objects actually removed from local retention after
explicit authorized retention policy; retained immutable objects are never free.
The foundation `Reserve` transition is pure: downstream must persist it before
I/O, serialize concurrent requests, and retain receipts even after failure.
It makes no claim to provide on-disk intake in WP10.

## Actual integration map and ownership

| Existing consumer | Required downstream behavior |
| --- | --- |
| `verifyEvent`, `StageProjectSnapshot`, `ReadArchive`, quarantine verification | Authenticate exact raw content/closure first; no mutable contribution authority in crypto or `store.go`. Retain raw actor ancestors and extract exact submission/decision/policy dependencies in `quarantine.go`. |
| `applyIssueMutation`, `executeIssueMutation`, legacy `gh_compat.go` issue dispatch and `github_import.go` import | One current operation authorization and admitted expected-head check; do not let compatibility aliases bypass it. |
| `applyResourceMutation`, `queryResource`, `executeIssueQuery` | Build issue/resource catalogs from admitted revisions, then apply tombstone visibility; retain exact references. |
| `cmdProposalOpen`, `cmdProposalRevise`, `cmdReview`, list/show/status, proposal lineage | Authorize live operation and derive terminal heads/lineage from admissible facts. Admission adds no review or decision evidence. |
| `discussion_commands.go`, `project_commands.go`, `workflow_commands.go` | Specialized queries/mutations use the same governed input; hidden discussion answers, project references and workflow definitions cannot leak through convenience readers. |
| `governance.go`, existing proposal evaluator and pipeline execution | Keep exact proposal/base policy/review/CI/acceptance/merge/isolation checks. Add current contribution permission before action, never reinterpret admission as native evidence. |
| `github_commands.go` issue exports; `resource_transfer.go`, `github_discussions.go`, `github_projects.go`, `github_workflows.go` | Govern human-facing export/query selection and import mutations; checkpoint recovery still binds original exact native evidence. Explicit full project archive/raw export is classified raw. |
| `audit_protocol.go` workflow-source selection | Govern current workflow selection; audit remains independently disabled and observational; historical signed audit evidence stays verifiable. |
| `cmdLog`, explicit `inspect --raw`, project archive/sync | Raw inspection/history explicitly labeled raw. Ordinary sync remains raw authenticated transport, never automatic admission. No source policy adoption or compatibility-all inbox fetch. |

WP10 owns only this document and `internal/contribution/{model,policy,record}.go`
and their tests. WP11 owns bounded exchange; WP12 owns live commands/adapters,
policy field and exact selected-authority checkpoint verification, governed
projections, contribution-ref reconstruction/backup adapters and the assigned
disjoint consumer files. WP13 owns public CLI proof and user guide. No extra source ownership is needed
for this foundation. Any real later shared-store need must receive canonical
allocation before editing. WP10 tests prove exported library boundaries only.

Historical native facts remain byte-exact. New readers preserve old facts;
explicit outer-format handling is required for new envelopes, and no new record or policy member is emitted when disabled. Released old readers
reject unknown new policy fields/kinds; they are not promised arbitrary forward
compatibility or governed contribution behavior. New envelopes cannot be
misrepresented as accepted native proposal decisions to old or new readers.

WP13 public acceptance must reuse `buildMigrationCLIBinary` or
`portableJourneyBinary` so the same contribution journey can exercise an injected
exact candidate package. Foundation tests never count as actual-package proof.

An example rule list is `[ {"operation":"issue.create","automatic":true},
{"operation":"issue.reply","automatic":true},
{"operation":"proposal.create","automatic":false}]`.
An absent rule denies admission. Optional `actors` restricts full signer keys;
`allowMaintainers` explicitly permits current native maintainers to edit another
author's resource. It never overrides native content/lineage verification.
Supported typed operations cover issue create/reply/edit, discussion create/reply/
edit, proposal create/revise, project and project-item create/edit, and workflow
create/edit. Defaults are 16 MiB wire, 32 MiB decoded, 4,096 objects, 2,048 events,
256 dependency depth and 30 seconds per request; window caps are 256 MiB,
32,768 objects, 16,384 events and 256 requests; retained caps are 1 GiB,
131,072 objects, 65,536 events and 1,024 requests. Wire and decoded maxima plus
4 KiB accounting metadata are charged conservatively without a refund on failure.
The static graph cap is 16,384 total envelope/native dependency records.
