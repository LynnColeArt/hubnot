# Guided and scripted project migration

`hn migrate` asks for the source, destination, explicit scope and mappings, shows
the actual configured capabilities and findings, and saves an inspectable plan
before asking for the exact answer `APPLY`. `NO` leaves the plan saved. Empty
answers, an unterminated answer and EOF never approve a finding or an apply.
`hn migrate help` lists the executable flags.

`hn migrate plan` is the same planning path with explicit arguments. It never
applies, including when every decision is supplied. `--guided` can fill missing
arguments and collect individual decisions. There is no blanket `--yes` flag.
`apply` and `resume` use the same guarded engine and require `--confirm-apply`,
or `--guided` followed by the exact `APPLY` answer.

## Local archive without accounts

Run from any directory. The session's parent directory must already exist; the
session itself must be absent. Keep sessions outside selected committed content,
for example below the resolved Git directory in `hn/migrations/NAME`.

```sh
hn migrate capabilities
hn migrate plan --source-kind local --source-repository /work/project \
  --ref refs/heads/main \
  --destination-kind archive --destination-repository /backups/project.hn \
  --session /work/project/.git/hn/migrations/export
hn migrate inspect --session /work/project/.git/hn/migrations/export
hn migrate apply --session /work/project/.git/hn/migrations/export --confirm-apply
hn migrate inspect --archive /backups/project.hn
```

Create the private session parent deliberately before running that example.
Repeat `--ref` for each full selected branch, tag or native actor ref. Select
native actor histories under `refs/hn/actors/` explicitly to retain those signed
facts and their required dependency closure. Selected committed history stays
exact; export is not a secret scrubber. Working files, unselected refs, private
signing identities, ambient credentials and private migration journals are not
exported.

Import to an existing destination Git repository:

```sh
hn migrate plan --source-kind archive --source-repository /backups/project.hn \
  --destination-kind local --destination-repository /work/new-project \
  --decision local/governance=preserve --session /private/import-plan
hn migrate inspect --session /private/import-plan
hn migrate apply --session /private/import-plan --confirm-apply
hn migrate imports --repository /work/new-project
hn migrate inspect --repository /work/new-project --snapshot sha256:FULL_DIGEST
```

A local import preserves historical governance and publishes native observations;
it does not replace destination policy, install signing identities or adopt audit
authority. `local/governance=preserve` explicitly records that choice. Other
source findings may need their own decisions. Initialize a destination signing
identity explicitly with `hn init` when authoring new native work. Imported
foreign authors, reviews, CI records and workflows remain descriptive content.

Archive and `saved` sources retain their complete immutable snapshot scope.
They reject `--ref`/`--kind` overrides. Local sources use explicit refs; provider
sources require explicit `--kind` selections. A comment selection is separate
from its parent issue selection.

## Select an installed provider and exact profile

`hn migrate capabilities` lists compiled provider kinds without credentials,
network requests or guessed support. `capabilities --source-kind ...` or
`--destination-kind ...` displays the actual selected instance's offline metadata
after construction. This is not proof of live access. Native/local/archive/saved
inspection needs no provider, account, credential or signing identity.

Every foreign source and destination requires explicit nonempty
`--SIDE-api-version`, `--SIDE-credential-mode` and `--SIDE-endpoint`. Guided mode
asks for these strings and the selected adapter validates them. Versions and
authentication are adapter-owned; the CLI does not infer them from capability
text or silently select a future default.

| Kind | Explicit API version/profile | Credential mode | Scope and limits |
| --- | --- | --- | --- |
| `github` | `2022-11-28` | `bearer` | `owner/repository`; issue/comment writes; open state and empty issue type only. |
| `gitlab` | `v4` | `private-token` | Explicit `/api/v4` root, including self-hosted prefixes; nested project selector; ordinary issues/notes. Slash-leading body lines, including fenced examples, refuse hosted recreation to avoid quick actions. |
| `forgejo` | `11` or `16` | `token` | Explicit `/api/v1` root; repository selector; numeric existing label IDs. |
| `gitea` | `1.24` or `1.27` | `token` | Separate tested profile; numeric label IDs; 1.27 updates require observed `content_version`. Neither Forgejo nor Gitea PATCH changes labels. |
| `bitbucket` | `2.0` | `anonymous`, `oauth2`, `api_token_bearer`, `api_token_basic` | Cloud repository/PR-conversation/workflow preservation. Removed issue APIs and destination PR recreation are unavailable in v1. No Jira substitution or app passwords. |
| `azure` | `services-7.1` or `server-7.0` | `bearer` or `pat` | Organization/collection and project are required. Repository is optional for Boards-only work; code, PRs and YAML require it. Explicit process/type/state mappings are required. |

Use `--SIDE-organization`, `--SIDE-project`, and `--SIDE-repository` as the
selected adapter requires. GitLab planning selectors are separate board IDs
(`--source-planning ID`), not repository names. `--source-workflow-ref` pins
workflow source; Bitbucket requires a full immutable commit hash. Retrieved
workflow source is inert. `--source-attachments FILE` accepts a strict bounded
JSON array of `AttachmentSelection` objects (`URL`, `Name`, `Parent`), with parent
keys in the shared migration schema. Attachment bytes are fetched only through
the selected adapter's origin and budget checks, never by following body links.

Supply API credentials afresh in a named environment variable with
`--source-token-env NAME` or `--destination-token-env NAME`. If omitted, only the
selected registration's `TokenEnv`, when present, is consulted. Azure has no
registered token environment default: name the variable explicitly. Other
providers' credentials are not read. Nonanonymous Bitbucket construction needs
genuine supplied credentials even for metadata. For `api_token_basic`, the
transient value is `email:api-token`; the email is not persisted separately.
`anonymous` requires an empty credential. The CLI never fabricates a token or
changes the chosen mode to make construction succeed.

The saved sealed `cli.json` is a strict bounded allowlist: schema, canonical plan
digest, normalized destination locator and the exact accepted API version and
credential mode. It contains no token, environment name or arbitrary config.
Missing, modified or mismatched metadata refuses apply. Session files contain
preserved source content and a local authentication key: keep them private.

## GitLab to GitHub with code and communications

The following illustrates already-present code and explicit ordinary issue/note
conversion. Set the selected credential variables before invoking it.

```sh
hn migrate plan --source-kind gitlab --source-endpoint https://gitlab.example/api/v4 \
  --source-repository group/project --source-api-version v4 \
  --source-credential-mode private-token --source-token-env SOURCE_API_TOKEN \
  --destination-kind github --destination-endpoint https://api.github.com \
  --destination-repository owner/project --destination-api-version 2022-11-28 \
  --destination-credential-mode bearer --destination-token-env DESTINATION_API_TOKEN \
  --kind issue --kind comment --source-git /work/project --ref refs/heads/main \
  --git-remote /remotes/project.git --map refs/heads/main=refs/heads/main \
  --map state:opened=open --map type:issue= --session /private/move-plan
```

This first read-only preview reports any outstanding decisions; supply each
specific `--decision FINDING_ID=preserve` or `=acknowledge` after inspecting its
message, or rerun with `--guided`. No usable plan is saved while decisions are
missing. IDs come from that preview; there is no wildcard acknowledgment.
Additional `--map label:OLD=NEW` and `--map assignee:OLD=NEW` selections are
explicit. An empty value is meaningful omission. `map` decisions are valid only
when the adapter's resulting finding is actually mapped. In guided mode choosing
`map` collects concrete mappings and replans against the same source observation.

Capabilities and findings distinguish `mapped`, `preserved`, `requires_mapping`,
`unsupported`, `missing_dependency` and `identity_conflict`. A preservation or
acknowledgment decision cannot fabricate an omitted issue/comment intent. Review
the saved intent list: a source with only preserved records does not imply
destination-native recreation. Retrieved unsupported PR/MR/review/planning/workflow
content remains in the saved snapshot. Inaccessible content is reported unavailable,
never claimed preserved. Inspect exact retrieved blob bytes with
`inspect --session DIRECTORY --blob sha256:FULL_DIGEST` (or `--archive FILE`).

Code uses the shared Git transport with explicit ref mappings and observed-old
guards. There is no implicit clone, fetch, force push or source deletion. API
credentials are not reused for Git. For private HTTPS Git, explicitly provide
`--git-origin HTTPS_ORIGIN --git-auth-env NAME` and optionally `--git-ca-file PATH`.
That variable contains the transient HTTP Authorization value for that origin.
Supply these again on apply/resume. Ambient Git helpers and redirects cannot
forward credentials to another origin. The saved `git_refs` intent owns the
remote/ref bindings; the CLI does not duplicate or replan them on resume.

## Apply, uncertainty and offline continuation

```sh
hn migrate inspect --session /private/move-plan
hn migrate apply --session /private/move-plan --confirm-apply \
  --destination-token-env DESTINATION_API_TOKEN
hn migrate resume --session /private/move-plan --confirm-apply \
  --destination-token-env DESTINATION_API_TOKEN
```

Resume reopens authenticated saved bytes and reconstructs the exact destination
profile with fresh credentials. It neither re-fetches the source nor calls its
snapshot planner. The engine resolves comment parents from confirmed issue
receipts, persists the exact bound payload before the one write, and reconciles
an uncertain write by exact positive evidence. Missing, ambiguous or inaccessible
proof requires manual recovery; retrying never authorizes a duplicate create.
Stale Git/provider baselines and hostile paths refuse before dependent writes.
There is no global transaction spanning independent APIs, Git and files: inspect
partial progress rather than deleting or editing checkpoints. Preserve the
original session when recovery needs investigation.

After provider servers and credentials are gone, use a `noforge` build:

```sh
hn migrate inspect --session /private/move-plan
hn migrate plan --source-kind saved --source-repository /private/move-plan \
  --destination-kind local --destination-repository /work/local \
  --decision local/governance=preserve --session /private/bring-home
```

Carry forward any source findings' explicit decisions from this preview. Apply
then inspect the digest listed by `imports`. To retain previously imported
foreign records alongside current code, select each imported snapshot explicitly
with `--source-kind imported --source-repository /work/local --snapshot DIGEST`,
and optionally `--source-git /work/local --ref refs/heads/main`. Archive that
composed observation through the same engine. Unselected imports are not claimed
as read. `nogithub` excludes GitHub alone; `noforge` excludes every network adapter
while keeping native collaboration, archives, imported records, Git and audits.

All CLI snapshots use the shared positive default budgets: 10,000 records,
1,024 blobs, 16 MiB per blob/manifest, 256 MiB attachment data, 100,000 Git objects,
16 MiB per Git object and 512 MiB total Git material. Limit exhaustion refuses
incomplete success. Archive admission rejects traversal, duplicates, links,
unexpected members, digest errors and incomplete native closure before visibility.
Imports do not activate workflows, permissions, audits, synchronization or any
background service. Existing `hn transfer` commands retain their own contracts.
