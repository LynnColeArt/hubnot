# Hubnot

Hubnot is an experiment in distributing collaboration with a Git repository.
Git distributes the work; Hubnot distributes the signed intent, discussion,
policy, and evidence around it. There is no Hubnot service or database: an
ordinary Git remote transports both code and `refs/hn/*` collaboration facts.

The active command and protocol namespace is `hn`. This is a deliberate
pre-user hard reset: there is no `nh` executable, alias, fallback reader,
migration, or actor-continuity bridge. This repository's tracked configuration
lives in `.hn/`. The obsolete `.nh/` directory is ignored; its earlier contents
remain in Git history. Legacy `.git/nh/**` and `refs/nh/*` state is also ignored.
Hubnot creates fresh identities and facts only in the active namespace.

The current beta prerelease supports signed issues, planning projects and items,
threaded discussions, versioned native workflows, immutable proposal
candidates and revisions, reviews, CI requests/results/logs, governance
decisions and merge facts, distinct device actors, policy amendments, selected
quarantined replication, exact shallow-history recovery, deliberate signed
agent memory with lexical and regex search, portable project migrations,
independent observational audits, optional contribution/moderation workflows,
and custom typed artifacts with an explicitly started local extension host.
It remains an experimental protocol, not a stable or hardened multi-tenant system.

## Project status

The initial operational-alpha roadmap is complete. Hubnot has used its own
published protocol to revise, independently sandbox-test, approve, accept, and
merge changes into public `main`. An ordinary Git remote carried the code and
signed collaboration refs; it did not supply Hubnot-specific governance or
CI services.

"Alpha complete" means the end-to-end protocol boundary documented here is
implemented and exercised. It does not mean production-ready: compatibility is
not yet stable, the security boundary is intentionally narrow, and the
deferred work in [Completed alpha boundary](#completed-alpha-boundary) is the
remaining roadmap. [0.1.0-beta.3](https://github.com/LynnColeArt/hubnot/releases/tag/v0.1.0-beta.3)
is a published beta prerelease, not completion of every beta acceptance gate.
The [prospective beta support contract](docs/beta-support.md) records the broader
compatibility, recovery and qualification goals; its full support promise has
not been adopted. The [current release notes](https://github.com/LynnColeArt/hubnot/releases/tag/v0.1.0-beta.3)
identify the tested bytes, platform scope and remaining limits; the
[beta.2 evidence](docs/releases/0.1.0-beta.2.md) remains a historical record.

## Download and install

[Hubnot 0.1.0-beta.3](https://github.com/LynnColeArt/hubnot/releases/tag/v0.1.0-beta.3)
is the current prerelease for Linux AMD64/ARM64 and Apple Silicon Macs.
Prebuilt executables require **Git 2.x** on your PATH. Running them requires no
Python, Go, Docker, or forge account.

The **exact downloadable Linux standard and native-only binaries passed bounded
native smoke checks** on AMD64 and on Golden Goddess's ARM64 host. Checks covered
installation, memory regex, migration, proposal ancestry and five extension
journeys per variant, plus a separate source-bound corruption regression.
They used supplied non-race harnesses without rebuilding the application.
Mac portability CI passed for the release source, but **runtime testing of the
exact beta.3 Apple Silicon release binaries is still pending**. Pink Menace's
[beta.2 report #33](https://github.com/LynnColeArt/hubnot/issues/33) qualifies the
earlier files. These are bounded observations, not an exhaustive suite or a new
full race-suite result.

**Mac Developer ID signing and notarization are pending.**
Browser-download/Gatekeeper installation remains uncertified. Intel Macs are
outside this prerelease's target set; beta.3 contains no Intel Mac packages.

### Debian and Ubuntu (APT)

The [Kinetic Actor package archive](https://packages.kinetic.actor/) provides
signed Debian packages for Linux **AMD64 and ARM64**. Set up the archive once:

```sh
sudo apt update
sudo apt install ca-certificates curl
sudo install -d -m 0755 /etc/apt/keyrings
curl -fsSL https://packages.kinetic.actor/kinetic-actor-archive-keyring.gpg | sudo tee /etc/apt/keyrings/kinetic-actor.gpg > /dev/null
sudo chmod 0644 /etc/apt/keyrings/kinetic-actor.gpg
echo 'deb [signed-by=/etc/apt/keyrings/kinetic-actor.gpg] https://packages.kinetic.actor/ ./' | sudo tee /etc/apt/sources.list.d/kinetic-actor.list
sudo apt update
sudo apt install hubnot
hn version
```

Archive key fingerprint: `6733 FF4F 9843 269A 7655 3614 28E8 F420 ED86 67BC`.
The `signed-by` setting scopes this key to the archive; no global `apt-key`
trust is needed.

For the **native-only** variant, use `sudo apt install hubnot-native` instead.
The two packages conflict: both provide `/usr/bin/hn`, so choose one. If an older
archive installation is earlier on your PATH, make sure it does not shadow
`/usr/bin/hn` (`command -v hn` shows the selected executable).

To upgrade an existing standard installation or list available versions:

```sh
sudo apt update
sudo apt install --only-upgrade hubnot
apt list -a hubnot
```

Use `hubnot-native` in those commands for a native-only installation. Beta.3's
Debian version is `0.1.0~beta.3-1`; the `~` preserves ordering before a future
final release. Older versions are retained in the archive.

These packages wrap the checksum-verified upstream static executables. APT
installs Git and CA certificates and recommends Bubblewrap for optional Linux
pipeline sandboxing. Installation enables no daemon or application policy;
Go and Python are not runtime dependencies. Package provenance and release
documentation are installed under `/usr/share/doc/hubnot/` or
`/usr/share/doc/hubnot-native/`. The archive publishes its
[package checksums and provenance](https://packages.kinetic.actor/package-manifest.json).

Archive installation, upgrades and variant switching were tested on Debian 12
and Ubuntu 24.04 AMD64. ARM64 Debian packages passed checksum/static architecture
checks; their installation has not yet been runtime-tested.

### Release archives (Linux and macOS)

| Platform | CPU | Standard | Native only |
| --- | --- | --- | --- |
| Linux | Intel / AMD 64-bit | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.1.0-beta.3/hubnot_0.1.0-beta.3_linux_amd64.tar.gz) | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.1.0-beta.3/hubnot_0.1.0-beta.3_linux_amd64_native-only.tar.gz) |
| Linux | ARM64 | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.1.0-beta.3/hubnot_0.1.0-beta.3_linux_arm64.tar.gz) | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.1.0-beta.3/hubnot_0.1.0-beta.3_linux_arm64_native-only.tar.gz) |
| macOS | Apple Silicon | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.1.0-beta.3/hubnot_0.1.0-beta.3_darwin_arm64.tar.gz) | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.1.0-beta.3/hubnot_0.1.0-beta.3_darwin_arm64_native-only.tar.gz) |

Standard builds include the optional GitHub, GitLab, Forgejo/Codeberg, Gitea,
Bitbucket Cloud and Azure DevOps adapters. Native-only builds omit every network
provider adapter and retain native collaboration, local/archive migration,
independent audits and local `hn gh issue` commands. Both variants install as
`hn`; choose one. Protected checks, automerge and audits remain independently
disabled by default.

Download [SHA256SUMS](https://github.com/LynnColeArt/hubnot/releases/download/v0.1.0-beta.3/SHA256SUMS)
alongside your archive and verify it before extraction. On Linux, use
`sha256sum --ignore-missing -c SHA256SUMS`. On macOS, use
`shasum -a 256 <archive.tar.gz>` and compare the matching checksum entry.
The [source archive](https://github.com/LynnColeArt/hubnot/releases/download/v0.1.0-beta.3/hubnot_0.1.0-beta.3_source.tar.gz)
and [build provenance](https://github.com/LynnColeArt/hubnot/releases/download/v0.1.0-beta.3/PROVENANCE.json)
are also available.
Extract your archive, enter its directory, then install:

```sh
mkdir -p "$HOME/.local/bin"
install -m 0755 hn "$HOME/.local/bin/hn"
export PATH="$HOME/.local/bin:$PATH"
hn version
```

Keep that PATH setting in your shell configuration if needed. The archives also
contain the MIT license, build provenance, and product documentation. See the
[changelog](CHANGELOG.md) for release history and platform qualification.

Published asset digests and downloaded bytes match the frozen bundle. All six
application packages passed static architecture/build-info checks; exact
runtime observations currently cover Linux only. The optional
[qualification test kit](https://github.com/LynnColeArt/hubnot/releases/download/v0.1.0-beta.3/hubnot-0.1.0-beta.3-test-kit.tar.gz)
is separate from the application. See the
[release notes](https://github.com/LynnColeArt/hubnot/releases/tag/v0.1.0-beta.3)
for source/compiler identities, test scope, historical staging files and Mac
distribution limits. Earlier release assets and qualification records remain
unchanged. Beta.3 includes the replication receipt-growth fix from
[PR #32](https://github.com/LynnColeArt/hubnot/pull/32); existing receipts are not
compacted.

## Build and verify

Building from source requires Go 1.26 or newer and Git 2.x. Docker and Python
are not required for the following build and test commands.

```sh
go build -trimpath -o hn .
# Bounded development feedback; the full suite remains an explicit deep check.
./.hn/actions/ci fast

# Omit GitHub only, or every network adapter. Native commands remain available.
go build -trimpath -tags nogithub -o hn-local .
go build -trimpath -tags noforge -o hn-native .
```

Ordinary source builds retain the development version string `0.0.1-dev`;
the release packager sets the requested product version at link time.

Packaging 0.1.0-beta.3 uses Go and Git alone, with no Python requirement. The
published archives were built with Go 1.26.4 from commit
`852f9d0bdfebb9635efdd122bad5f9d546d7c15b`. Build the Go-only packager and
select that exact revision to reproduce them:

```sh
go build -trimpath -o hn-release ./cmd/hn-release
./hn-release --rev 852f9d0bdfebb9635efdd122bad5f9d546d7c15b --version 0.1.0-beta.3 --targets linux/amd64,linux/arm64,darwin/arm64 --out NEW_DIRECTORY
```

Use a clean checkout; tracked changes are refused even with an explicit revision.
The output directory must not exist. Optional `--source DIRECTORY`,
`--targets linux/amd64,linux/arm64,darwin/amd64,darwin/arm64` and
`--variants standard,native-only` select declared inputs. The packager isolates
the exact committed source, disables automatic VCS stamping, uses no shell or
module network, and writes normalized binary/source archives, `BUILD.json`,
`PROVENANCE.json` and `SHA256SUMS`. It never tags or publishes. Current
native-only packages exclude all network adapters with `nogithub,noforge`;
the published first release's native-only assets excluded GitHub only.

The beta.3 release matrix was packaged on Linux. Native Apple Silicon
packager execution was separately demonstrated for its native target pair;
Windows packager-host execution remains unverified. CLI target cross-compilation
requires separate runtime evidence, as qualified above. The packager's four-target
default is broader than beta.3's three packaged targets; the command above
explicitly excludes Intel Macs. See [release packaging](docs/releasing.md).
The immutable first release's `build-release.py` still requires Python 3.12+,
Git and Go 1.26.4 to
reproduce its historical archives; its tags and assets stay unchanged.
Windows CLI distribution remains outside the current target set.

The end-to-end acceptance scenarios compile the CLI and exercise the
self-hosted governance/CI loop, repository-native agent memory, and rich issues
through real Git repositories:

```sh
go test -count=1 -run 'TestOperational(SelfHostingAlpha|AgentMemory)$' ./...
go test -count=1 -run '^TestOperationalRichIssue' -v ./...
```

Run the full local release checks with:

```sh
go test -race -timeout 20m ./...
go vet ./...
git diff --check
```

Bubblewrap is used by the default Linux CI executor when you choose to execute
a pipeline. It is not required for identity, policy, proposal, review,
replication, inspection, or verification commands. The explicit unsafe host
executor remains available for controlled debugging. macOS has no bundled safe
CI execution backend; host execution requires the existing explicit
`--allow-unsafe-host-execution` opt-in.

## Portable projects and independent audits

[Guided and scripted migration](docs/migration-cli.md) uses one saved
plan/apply/resume engine for local repositories, archives and explicitly selected
providers. Inspect capabilities, source bytes and loss/mapping findings before
writing; apply requires `--confirm-apply` or the exact guided answer `APPLY`.
A timeout or negative lookup never authorizes repeating an uncertain create.
[Snapshots](docs/project-migration.md) retain selected Git history, exact signed
native facts and retrieved attributed foreign records/blobs. Import preserves
destination governance and private identity state; source history grants no
native roles. Offline inspection and local continuation work with `noforge`.
The [provider guide](docs/github-interoperability.md) distinguishes tested
profiles, native mappings, raw source preservation and unavailable recreation.
Existing explicit `hn transfer` behavior remains available.

[Independent audits](docs/audits-v1.md) require optional committed audit roles,
a separately adopted private authority anchor, owner-authorized rules, an
explicit request and a separate execute command. For example, after declaring
the roles and committing `.hn/pipelines/check.json`:

```sh
hn audit authority set --ref refs/heads/main --expect absent
hn audit rule authorize check --source pipeline --at HEAD
hn audit request --at CODE_REV --rule check --operation inspect-release
hn audit execute FULL_REQUEST --backend sandbox
hn audit show FULL_RESULT
hn audit follow-up FULL_RESULT --operation triage-release
```

Reads need no signing key. Audits can inspect old or already merged code without
a proposal. Passing and failing audit results neither block nor qualify proposal
CI, reviews, acceptance or merge. Execution preserves the sandbox default;
`--backend host` requires `--allow-unsafe-host-execution` on every invocation.
Follow-ups are explicit linked native issues, deduplicated across retries and
later results without overwriting operator edits. Concurrent independent
publishers remain visible; there is no distributed exactly-once guarantee.
Imports, requests, initialization, upgrades and merges start no audit worker.
Protected checks, automerge and audits are independent opt-ins, all disabled
unless explicitly configured. No scheduler, service or provider account is
required.

## Start a repository

Run these commands inside an existing Git repository. For a new project, create
and enter its directory, then run `git init` before `hn init`.

```sh
hn init --name "Alice's device"
hn identity public
hn issue open --body "The description" "An issue title"
hn sync origin
```

`hn init` creates a local Ed25519 identity below `.git/hn/`. Private keys and
local replication selections are never Git objects and are not cloned. Public
events are exact-byte signed and stored in one append-only Git history per
actor:

```text
refs/hn/actors/<full-actor-fingerprint>
```

## Issues that grow with the work

Start with a title and add structured descriptions, criteria, labels,
assignees, relationships and namespaced metadata when useful. Edits, close and
reopen append signed revisions under the same opening identity. Concurrent
heads remain visible until explicitly resolved; assignment and status grant
no execution or review authority.

```sh
hn issue list
hn issue show <issue-id>
hn issue heads <issue-id> --json
hn issue help
```

[Rich issue guidance](docs/issues-v1.md) covers full-state JSON input,
expected-head guards, actor-scoped operation retries, bounded pages and
conflict recovery. The [runnable consumer](examples/issue-consumer/README.md)
creates, reads, revises and retries demonstration work using only the public
CLI. No Beads migration or consumer-specific runtime integration is performed.

## Projects, discussions and workflows

Native resources are signed Git facts and remain usable with no provider:

```sh
hn project help
hn discussion help
hn workflow help
```

[Projects](docs/projects-v1.md) contain typed planning fields and independently
revisioned items pinned to an exact schema. [Discussions](docs/discussions-v1.md)
keep independent replies and accepted answers pinned to exact reply content.
[Workflows](docs/workflows-v1.md) store native `hn.pipeline/0` definitions and
inert foreign source documents. Materializing or requesting a workflow does not
execute it; the existing explicit policy/proposal/runner path governs execution.
These new native kinds require a current reader; older alpha executables may
reject them without changing their signed bytes.

## Familiar local commands and optional transfers

`hn gh issue` implements a documented local subset using the same native issues:

```sh
hn gh issue create -t "Improve setup" -b "Document the first successful run"
hn gh issue list --state all --json id,title,state,conflict
hn gh issue view <native-issue-id> --comments
```

It never invokes `gh`, contacts GitHub or reads forge credentials. Native IDs
remain authoritative. Optional explicit transfer is separate:

```sh
hn transfer capabilities --provider github
hn transfer import --provider github --project OWNER/REPO --key 42 --dry-run
hn transfer import --provider github --resource discussion --project OWNER/REPO \
  --key 7 --map history=snapshot --dry-run
```

The [interoperability guide](docs/github-interoperability.md) covers the exact
compatibility flags/JSON, all four resource mappings, credentials, bounded
preflight and uncertain-write recovery. GitHub is an optional first adapter;
there is no mandatory account, service, SDK, or unmodified-`gh` API bridge.
Imported facts remain usable after transport removal. Foreign workflows and
Projects automation rules are not assumed equivalent to native execution.

## Optional custom types and local extensions

`hn artifact` stores signed custom documents, ordered lists and tickets with pure
versioned schemas. Generic reads, bounded literal/regex search and public exchange
work without a plugin. Domain rules and the meaning of custom fields stay with
the consumer; schema validation or signed content does not activate policy,
doctrine, prompts or execution.

`hn extension` can explicitly register, enable and run a trusted local executable
over stdio with declared capabilities and repository grants. Cloning a declaration,
reading a record or running an ordinary native command never starts an extension.
Extensions are same-user processes, not a hostile-code sandbox. The optional
fake-tracker, charter-model and inventory examples demonstrate generic integration;
they do not embed Spec Kitty doctrine or provide production enterprise connectors.

```sh
hn artifact help
hn extension help
```

Start with the [plugin quickstart](docs/plugin-quickstart.md). See
[portable artifacts](docs/artifacts-v1.md), [the extension contract](docs/extensions.md)
and [consumer recipes](docs/extension-consumers.md) for the detailed references.

## Optional contribution and moderation workflow

Contributors can hand over signed issues, replies, discussions and code proposals
as bounded local files without upstream push access or a provider account.
Receiving, admitting and hiding content use explicitly configured project
authority; admission grants no review, runner or merge authority. The workflow
starts no listener or background service and leaves immutable history intact.

See [contributor and moderator guidance](docs/contributions.md) for setup,
standalone operation, moderation, capacity and recovery. Optional hosting or
standing acceptance is not inferred from this file-based workflow.

## Distinct device actors

Initialize another clone independently, then exchange only the output of
`hn identity public`. The existing actor authorizes the new public key and the
new actor accepts that exact authorization:

```sh
hn identity authorize --relationship device \
  --actor <full-new-actor> \
  --public-key <new-public-key>
hn sync origin

# In the independently initialized target clone, after selected sync:
hn identity accept <full-authorization-event-id>
hn sync origin
hn identity list
```

The relationship is descriptive. It does not add the new actor to any policy
role. `hn identity rotate` performs the same two-sided protocol locally with a
`successor` relationship and retryable transaction state; it is planned
rotation while the predecessor key is available, not lost-key recovery.

## Inspect and amend policy

Project governance lives in `.hn/policy.json`. Inspection prints complete
trust-bearing actor IDs. An amendment is an ordinary candidate, and only the
exact policy bytes from its signed base govern it:

```sh
hn policy show main
hn policy check --base main --file .hn/policy.json
hn policy check --base main --head HEAD

hn proposal open --base main --head HEAD \
  --body "The base policy governs this amendment." \
  "Amend collaboration policy"
```

Continuity relationships and replication selections never grant maintainer,
reviewer, runner, or decision authority. Only the explicit full actor lists in
the candidate's base policy do.

## Select and synchronize facts

Save exact full actor/candidate selections and positive local budgets before
synchronizing an unfamiliar remote:

```sh
hn replication select origin \
  --actor <full-maintainer-actor> \
  --actor <full-reviewer-actor> \
  --proposal <full-candidate-event-id> \
  --max-events 10000 \
  --max-objects 100000 \
  --max-object-bytes 16777216 \
  --max-attachment-bytes 1048576 \
  --max-total-bytes 268435456
hn replication show origin
hn sync origin
```

Each selected ref is fetched into a separate bare quarantine repository,
measured and verified, then promoted to `refs/hn/remotes/<remote>/*` in one
atomic ref transaction. Independently valid selections may promote when
another selected history fails. Standard Git can download a selected pack
before Hubnot measures it, so these are hard validation, promotion, and
retention limits—not portable pre-download network quotas.

A trust-sensitive command in a depth-limited clone reports a full missing ID
and exact recovery action. Recovery is explicit and uses the saved selection:

```sh
hn sync origin --recover-shallow
```

This never performs a global unshallow and never silently adds a selector.

## Record and recall agent memory

Hubnot can carry deliberate project cognition without a hosted memory
service. `hn-memory/0` is separate from the collaboration protocol: each actor
owns append-only memory streams under `refs/hn/memory/*`, while private keys,
replication selections, and the rebuildable lexical index stay below
`.git/hn/` and are never cloned.

```sh
hn memory record --kind decision --at HEAD --applies descendants \
  --topic architecture --evidence git:$(git rev-parse HEAD) \
  --content "Keep memory streams independent from actor event chains."

hn memory handoff --at HEAD --applies descendants --input handoff.json --json
hn memory supersede sha256:<full-memory-id> \
  --kind decision --at HEAD --applies descendants \
  --content "Replacement decision with current rationale."

hn replication select origin --memory sha256:<full-stream-id>
hn sync origin
hn memory index rebuild
hn memory recall --at HEAD --topic architecture --json
# Explicit regex fallback when lexical recall misses the wording:
hn memory grep --at HEAD --topic architecture --json 'stream[-_ ]isolat.*'
```

Default recall is policy-qualified, active, local-only, and bounded to 20
records and 65,536 encoded content bytes. Every item retains its full IDs,
anchor, lifecycle edges, evidence, applicability, signature, trust class, and
content digest. Memory content and handoff next actions are untrusted inert
data—not instructions, truth, policy authority, or permission to act.
Regex grep shares the verified recall filters and bounded pages; it is an explicit
choice, not automatic fallback. See [memory regex semantics](docs/memory-v0.md#explicit-regex-fallback).

Add sorted `memory.trustedActors` and `memory.trustedKinds` to the exact commit's
`.hn/policy.json` to qualify default recall. Without it, valid signed memory is
explicitly inspectable but not trusted by default. Retraction preserves an
auditable fact; neither retraction nor deleting the private index erases
replicated Git objects.

## Proposals, CI, decisions, and merge

```sh
hn proposal open --base main --head feature \
  --body "Please review this exact change." "Add the feature"
hn run request <full-candidate-event-id> test
hn sync origin

# In a selected participant clone:
hn run execute <full-run-request-event-id>
hn review <full-candidate-event-id> --approve \
  --body "The fetched candidate is correct."
hn sync origin

# In a maintainer clone:
hn proposal status <full-candidate-event-id>
hn decide <full-candidate-event-id> --accept
hn merge <full-candidate-event-id>
hn sync origin                    # publish collaboration refs
git push origin main:main         # publish the primary branch separately
```

If Git merged the exact proposal head but recording `proposal.merged` failed,
retry `hn merge <full-candidate-event-id>`. The repair path does not merge
again: it reloads the current policy and evidence, requires one unique
first-parent merge commit that directly names the proposal head, and appends
only the missing signed fact.

Pipeline definitions are repository JSON files. A step can invoke an installed
tool or an executable tracked in the candidate, without a shell unless the
command itself is a shell:

```json
{
  "version": "hn.pipeline/0",
  "steps": [
    {
      "name": "Tests",
      "command": "./.hn/actions/test",
      "args": ["--all"],
      "timeoutSeconds": 300
    }
  ]
}
```

On Linux, the default executor uses Bubblewrap with separate user, PID,
network, IPC, UTS, and cgroup namespaces; dropped capabilities; no host home;
read-only system tools; and a writable generated checkout. It exposes no
external network interface. The Bubblewrap executable and sandbox `PATH` are
resolved from the same fixed canonical system directories, never the invoking
shell's ambient `PATH`. The unsafe host backend requires both flags on every
invocation:

```sh
hn run execute <full-run-request-event-id> \
  --backend host \
  --allow-unsafe-host-execution
```

If Git reports a merge conflict, resolve it outside Hubnot and publish a new
immutable revision. The predecessor and all prior evidence remain unchanged;
the revision needs its own exact review, CI, and acceptance evidence:

```sh
hn proposal revise <full-predecessor-candidate-id> \
  --base <full-resolved-base-commit> \
  --head <full-resolved-head-commit> \
  --body "Resolve the merge conflict"
hn sync origin
```

A runner refuses to publish a second result for the same exact request unless
the operator deliberately passes `--rerun`; the replacement is another signed
fact from that runner rather than a mutation of earlier bytes.

## Command surface

```text
hn init [--name NAME]
hn identity show|list|public|authorize|accept|rotate
hn issue open|revise|resolve|close|reopen|comment|list|show|heads|history|graph|operation
hn gh issue create|list|view|edit|comment|close|reopen
hn project help
hn project-item help
hn discussion help
hn discussion-reply help
hn workflow help
hn artifact help
hn extension help
hn contribution authority set|show; prepare|receive|pending|inspect|decide|archive|restore|baseline|retained|intake|rollover
hn migrate help
hn audit help
hn transfer import|export|capabilities --provider PROVIDER
hn github import|export --repo OWNER/REPO
hn proposal open|revise|list|show|status
hn policy show [REV]
hn policy check --base REV <--head REV|--file PATH>
hn review PROPOSAL <--approve|--request-changes> [--body TEXT]
hn run request|list|show|logs
hn run execute REQUEST [--backend sandbox|host] [--allow-unsafe-host-execution] [--rerun]
hn runner once|watch --accept-pipeline NAME --accept-actor ACTOR
hn decide PROPOSAL <--accept|--reject> [--body TEXT]
hn merge PROPOSAL
hn automerge PROPOSAL --rule NAME [--execute]
hn memory record|handoff|supersede|retract|challenge|show|recall|grep|index
hn replication select|show [REMOTE] [--memory STREAM]...
hn sync [REMOTE] [--recover-shallow]
hn log
```

Trust-bearing commands require full actor fingerprints and full
`sha256:<64-hex>` event IDs. Short IDs are display conveniences only.

## Opt-in check protection and automerge

Both capabilities are disabled unless a repository explicitly configures them in
`.hn/policy.json`. Initialization, upgrades and imports do not turn either on.

| Capability | Default | Explicit opt-in |
| --- | --- | --- |
| Protected checks | Disabled | Require designated review for changes to selected check files |
| Automerge | Disabled | Permit a named rule to merge an already-accepted candidate |
| Background operation | None | Run the command yourself or through your own scheduler |

Protection and automerge are independent: either may be enabled alone. They use
native actor identities, signed evidence and ordinary Git; neither requires
GitHub, `gh`, a service, or a provider account.

```sh
hn automerge <full-candidate-event-id> --rule docs-only
hn automerge <full-candidate-event-id> --rule docs-only --execute
```

The first command previews without a signing key or mutation. The second requires
an explicitly permitted maintainer identity and current reviews, CI passes and
acceptances. It merges locally; `hn sync` and `git push` remain separate actions.
Automerge requires a full repository without active partial/promisor configuration,
a clean target equal to the candidate's signed base, and no unfinished Git operation.
Policy amendments and protected-check changes use the manual merge path.

See [optional policy rules](docs/governance-v0.md#optional-check-protection-and-automerge)
for configuration, protected baselines, execution boundaries and recovery.

## Completed alpha boundary

The operational-alpha column records the original implemented boundary. Newer
optional contribution/moderation, regex memory and custom artifact/extension
features are described above. The deferred column records remaining work;
alpha completion and beta prerelease publication do not promise those features.

| Area | Operational alpha | Explicitly deferred |
| --- | --- | --- |
| Policy | Show, validate, diff, and govern amendments from exact base bytes; opt-in protected checks and explicit native automerge | Background merge queues and implicit role migration |
| Identity | Distinct actors, mutual device/successor facts, local keyring, retryable planned rotation | Lost-key recovery without a usable protected backup, compromise, social/organizational recovery, or concurrent writers sharing one actor key |
| Replication | Exact selections, quarantine, positive budgets, validation, atomic accepted refs, compatibility-all | Portable hard pre-download quotas, a network moderation service, selective deletion, and global redaction |
| CI | Repository-defined actions, default Bubblewrap runner, explicit unsafe host fallback | Secrets, configurable network access, strong CPU/memory/disk/process quotas, and portable/container backends |
| Memory | Signed streams, exact anchors/evidence, lifecycle projection, bounded lexical recall, disposable local index, and selected transport | Automatic capture, embeddings, semantic truth, autonomous action, federation, moderation, redaction, retention enforcement, and erasure guarantees |
| Product | CLI and Git-native signed facts | Notifications, global cross-resource search, web UI, discovery, and an adopted stable compatibility promise |

Published immutable facts may be superseded by new facts, but Hubnot cannot
promise global erasure from every replica.

## Documentation

- [Changelog and next-release requirements](CHANGELOG.md)
- [Current beta.2 downloads, evidence and platform decisions](docs/releases/0.1.0-beta.2.md)
- [Prospective beta support contract](docs/beta-support.md)
- [Historical data compatibility](docs/compatibility.md)
- [Backup and recovery](docs/recovery.md)
- [Interrupted operations and upgrades](docs/operation-recovery.md)
- [Guided and scripted project migration](docs/migration-cli.md)
- [Optional contribution and moderation](docs/contributions.md)
- [Rich issues](docs/issues-v1.md)
- [Projects and typed items](docs/projects-v1.md)
- [Discussions and pinned answers](docs/discussions-v1.md)
- [Native workflows and explicit execution](docs/workflows-v1.md)
- [Optional forge interoperability and local gh subset](docs/github-interoperability.md)
- [Optional local extension host](docs/extensions.md)
- [Plugin quickstart: a custom ticket from setup to removal](docs/plugin-quickstart.md)
- [Portable typed artifacts](docs/artifacts-v1.md)
- [Optional consumer examples](docs/extension-consumers.md)
- [Protocol and storage](docs/protocol-v0.md)
- [Agent Memory Protocol and operator guide](docs/memory-v0.md)
- [Agent memory safety model](docs/memory-safety.md)
- [Identity continuity and keyring safety](docs/identity-v0.md)
- [Governance and policy amendments](docs/governance-v0.md)
- [Selected replication and shallow recovery](docs/replication-v0.md)
- [CI and runner threat model](docs/ci-v0.md)
- [Hubnot's own CI: fast feedback and native stress runs](docs/project-ci.md)
- [Hosted Git compatibility](docs/host-compatibility.md)
- [Operational self-hosting proof](docs/self-hosting-alpha.md)

## License

Hubnot is licensed under the [MIT License](LICENSE).
