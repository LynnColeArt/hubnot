# Backup and recovery

Hubnot currently ships as an operational alpha. These procedures are rehearsed
locally by `TestBetaRecoveryJourneys` with standard and `noforge` binaries; they
are not a claim of beta qualification, hosted disaster recovery or key escrow.
Use the compatible `hn` version, Git, and a standalone repository. Commands below
use full actor fingerprints, full memory stream IDs and exact Git object IDs.

## What to retain

Keep three things separate:

| Material | Purpose | Handling |
| --- | --- | --- |
| Code plus explicitly selected `refs/hn/*` histories | Keyless public reconstruction | Public bundle, ref manifest and independently retained digest |
| Completed private keyring | Exclusive restoration of an existing signer | Offline protected storage; owner-only directories/files; never Git or a public archive |
| Local configuration, authority and interrupted-operation journals | Local choices and recovery bindings | Inventory separately; preserve uncertainty and originating context; never infer authority or blindly replay |

Ordinary clone omits custom collaboration and memory refs. A working-tree copy,
code-only bundle, provider export or public migration archive is not a private
backup. Include every actor/candidate/memory dependency needed by the selected
history. Unmerged proposal code lives under `refs/hn/proposals/<event-digest>`;
retain it as well as its signed author history. Enumerate accepted observations
under `refs/hn/remotes/*` if those contain the only retained copy. An excluded
history will not be reconstructed. Git object reachability alone does not
preserve the names of custom refs.

Do not back up a live transaction by copying arbitrary files mid-write. Stop
writers, record the compatible executable/version and completion state, and
retain a coherent public snapshot and its matching completed keyring. Keep
multiple dated snapshots and test restoration in an isolated repository. A
private backup predating public history is usable only after recovering and
verifying the latest signing chains.

## Public snapshot and keyless reconstruction

This example selects one branch, one collaboration actor and one memory stream.
Substitute the exact selectors; add other required actor/candidate/stream refs
explicitly. Store the bundle and manifest outside the repository being backed
up, in a directory you control. This sequence does not initialize a signer.

```sh
ACTOR=<full-64-hex-actor>
STREAM=<64-hex-memory-stream-digest-without-sha256-prefix>
ACTOR_REF=refs/hn/actors/$ACTOR
MEMORY_REF=refs/hn/memory/$ACTOR/$STREAM

git for-each-ref --format='%(refname) %(objectname)' \
  refs/heads/main "$ACTOR_REF" "$MEMORY_REF" > /safe/backup/refs.txt
git bundle create /safe/backup/public.bundle \
  refs/heads/main "$ACTOR_REF" "$MEMORY_REF"
git bundle verify /safe/backup/public.bundle
# Retain a digest through a separate trusted channel/storage medium.
sha256sum /safe/backup/public.bundle > /safe/backup/public.bundle.sha256

git clone --branch main /safe/backup/public.bundle restored
cd restored
git fetch --no-write-fetch-head /safe/backup/public.bundle 'refs/hn/*:refs/hn/*'
git for-each-ref --format='%(refname) %(objectname)' \
  refs/heads/main "$ACTOR_REF" "$MEMORY_REF"
hn issue show sha256:<full-retained-issue-id> --json
hn memory show sha256:<full-retained-memory-id> --json
hn identity list
hn identity show --json  # expected to refuse: no local signer
```

Compare every selected ref target with the manifest and retain the exact old
objects, including `event.json`/`memory.json`, signatures and attachments. Use
`git rev-list --objects` and `git cat-file --batch` to compare their exact bytes
between the retained and restored repositories; `git fsck` alone does not prove
signature validity. Public reads verify supported signed records. Recall can
create a disposable lexical cache, so it is not a byte-for-byte private-state
restore test. No public read grants a policy role or adopts audit authority.

The direct `refs/hn/*` fetch above is for a locally controlled, authenticated
backup whose selected refs and digest you retained. A digest shipped beside an
untrusted bundle proves no origin. For an untrusted remote use an exact saved
selection and `hn sync` quarantine verification, inspect its accepted outcomes,
and apply the documented replication budgets. Do not run imported hooks,
pipelines, workflows or scripts to inspect a backup. These tests need no forge
credentials, hosted service, provider CLI, daemon or runner execution.

## Private backup and exclusive disaster restoration

Normal additional devices create **distinct actors**. Copying a private key to
two active devices is unsupported. A same-actor restore is only disaster
recovery after the previous writer is fenced: shut it down and remove its access
to the key and repository, including scheduled jobs, agents and other worktrees.
If fencing or freshness cannot be established, use a fresh actor instead. Hubnot
cannot detect an unknown disconnected second signer, invalidate its private key
or erase facts it has already signed.

For a completed, quiescent standalone keyring, resolve the real private Git
location and copy just the active pointer and its selected identity record. The
following shell copy corresponds to the owner-only regular-file copy exercised
by the rehearsal. Validate that source paths are regular, non-symlink files in
owner-only directories before copying. Do not use these commands on a keyring
with unfinished rotation or command journals.

```sh
GIT_PRIVATE=$(git rev-parse --absolute-git-dir)
# ACTOR is the full actor printed by hn identity public.
umask 077
mkdir -p /safe/private/identities
cp -p "$GIT_PRIVATE/hn/active" /safe/private/active
cp -p "$GIT_PRIVATE/hn/identities/$ACTOR.json" /safe/private/identities/
chmod 700 /safe/private /safe/private/identities
chmod 600 /safe/private/active /safe/private/identities/"$ACTOR.json"
```

Protect this material with your offline secret-backup controls. Never attach it
to an issue, commit it, print it as evidence, or copy it into a public archive.
Check permissions and symlinks on the actual destination; Hubnot refuses unsafe
private directories, files and key material instead of repairing them silently.
A backup may also need a legacy `identity.json` for investigation; explicit
legacy migration preserves that signer, but is not permission to merge two
keyrings. Preserve interrupted `rotation.json`/`rotation-command.json` together
with all bound records privately and follow the original compatible rotation
recovery procedure. Do not initialize a new actor over an interrupted restore.

After fencing, reconstruct the public repository first. Restore the selected
completed keyring under its resolved private Git directory with mode `0700`
directories and mode `0600` regular files, then inspect `hn identity public` and
`hn identity show --json`. They must name the backed-up actor; do not copy audit
anchors, caches, automerge state or arbitrary journals as part of this key copy.

Recover current accepted history before signing. If the public backup is old,
configure a trusted retained remote and save an exact selection:

```sh
hn replication select origin --actor "$ACTOR" --memory "sha256:$STREAM"
hn sync origin
git rev-parse "refs/hn/remotes/origin/actors/$ACTOR"
git rev-parse "refs/hn/remotes/origin/memory/$ACTOR/$STREAM"
hn identity list
hn memory show sha256:<full-retained-memory-id> --json
```

Inspect actual accepted refs even when `sync` reports failure: a stale memory
push can fail **after** the newer incoming history was accepted. Do not delete
that accepted evidence, rerun blindly, or infer that nothing was imported from
the command exit status. A publication error does not authorize an overwrite.

Common collaboration and memory writers refuse a missing, stale or divergent
signing root when conflicting accepted history is already known locally. They
compare verified signed IDs at every sequence; an equal-length fork is not
fresh. Accepted older prefixes and unrelated actors/streams do not prevent
valid appends. The guard checks before the common writer writes objects or refs
and reads signatures without importing embedded audit packs. This is a local
consistency guard, not a distributed lock or automatic repair. Command-specific
preparation may have its own documented effects (for example normal audit
history reads import verified snapshots); do not treat every command as a
transactional rollback of all prior preparation.

For a missing or stale **prefix**, verify that the accepted chain belongs to
this exact signer/stream, starts at sequence 1, and continues every signed
predecessor through the newest head. Compare all available retained sources and
confirm there is no later owned or externally retained work. Record both the
verified recovery target and the expected old local ref. Only then deliberately
restore signing ownership with Git's expected-old compare-and-swap:

```sh
# Fill these from the independently verified heads and current owned refs.
# An empty OLD value asserts the ref is absent, including SHA-256 Git repos.
git update-ref "$ACTOR_REF" "$VERIFIED_ACTOR_HEAD" "$OLD_ACTOR_HEAD"
git update-ref "$MEMORY_REF" "$VERIFIED_MEMORY_HEAD" "$OLD_MEMORY_HEAD"
# Continue through ordinary commands, which sign successors of those roots.
hn identity authorize --relationship device \
  --actor "$NEW_DEVICE_ACTOR" --public-key "$NEW_DEVICE_PUBLIC_KEY"
hn memory record --kind decision --at HEAD --applies descendants \
  --stream "sha256:$STREAM" --content "Recovered exclusive writer" --json
```

No command silently adopts an accepted remote head as a signing root. A CAS
failure means re-inspect; do not force it. If histories diverge, preserve both
signed histories, fence all possible writers and investigate. Do not pick the
largest sequence, newest timestamp or shortest branch, rewrite old signatures,
or discard accepted evidence to unblock a signer. The rehearsal refuses a
restored divergent root; it does not advertise automatic fork reconciliation.

## Lost key, fresh actor and authority

Without a private backup the old signer cannot be recovered. In a separate
public reconstruction run `hn init --name 'Replacement device'`; inspect the
new fingerprint and keep it distinct. The new actor can make ordinary signed
contributions, but identity continuity, key possession, imported facts and
matching display names confer no maintainer/reviewer/runner/audit role.

A remaining maintainer can grant the new actor a role through a normal policy
amendment proposal evaluated under the **old base policy**, with its existing
review, CI, acceptance and merge requirements. The proposal cannot grant itself
permission to be accepted. The rehearsal proves refusal by the fresh actor,
then acceptance and merge by remaining old-policy authority before the new role
works. If the lost key was the sole required authority and no authorized backup
exists, the original policy has no native recovery path. Creating a new project
or deliberately establishing a new governance history is an explicit external
organizational decision; do not describe it as recovering old authorization.

Audit authority is a separate local choice. Public recovery and private key
restoration do not install it, even when old signed audit facts survive:

```sh
hn audit authority show --json  # absent until explicitly adopted
hn audit authority set --ref refs/heads/main --expect absent --json
hn audit authority show --json
```

Adopt only after inspecting that branch and its exact current policy. Adoption
selects where policy is read; it grants no role. Changing an existing anchor
requires the exact prior selection digest, not `absent`. Historical audit packs
and copied anchor files are not proof of current authority.

## Worktrees, exclusions and uncertain external operations

The rehearsals cover standalone repositories. In a linked worktree `.git` is a
file; `git rev-parse --absolute-git-dir` and `git rev-parse --git-common-dir` may
name different directories. Refs/object storage can be shared while identities,
anchors and journals are worktree-local. Inventory each actual location and
fence **all** writers sharing those refs. Copying the main checkout's `.git/hn`
is not a supported recipe for restoring every linked worktree; reconstruct a
standalone repository or perform a separately reviewed worktree recovery.

Public bundles exclude private keys, active pointers, local audit authority,
replication selections/acceptance journals, automerge leases, disposable memory
indexes, provider credentials and migration/export journals. Working-tree
uncommitted files and ignored files also need a separate intentional backup.
A bundle can contain old committed policy, scripts or accidental historical
secrets: selection and public suitability require review before distribution.

Migration and provider-write journals record exact destinations, remote IDs and
uncertain effects. Preserve them privately with the originating compatible
binary, source identity, session and destination binding. A response lost after
a remote create does not prove the object absent. Do not delete/reset a journal,
copy it to a new destination, or blindly replay it as a fresh operation. Follow
that operation's explicit inspection/reconciliation procedure; refuse to create
a duplicate while the prior external outcome remains uncertain. More detailed
interruption journeys are tracked separately in this beta mission.
