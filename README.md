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

The current operational alpha supports signed issues, planning projects and items,
threaded discussions, versioned native workflows, immutable proposal
candidates and revisions, reviews, CI requests/results/logs, governance
decisions and merge facts, distinct device actors, policy amendments, selected
quarantined replication, exact shallow-history recovery, deliberate signed
agent memory, portable project migrations and independent observational audits.
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
post-alpha roadmap.

## Download and install

[Hubnot 0.0.1-dev](https://github.com/LynnColeArt/hubnot/releases/tag/v0.0.1-dev)
is the first packaged operational-alpha prerelease. Prebuilt executables require
**Git 2.x** on your PATH. Running them requires no Python, Go, Docker, or forge
account.

| Platform | CPU | Standard | Native only |
| --- | --- | --- | --- |
| Linux | Intel / AMD 64-bit | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.1-dev/hubnot_0.0.1-dev_linux_amd64.tar.gz) | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.1-dev/hubnot_0.0.1-dev_linux_amd64_native-only.tar.gz) |
| Linux | ARM64 | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.1-dev/hubnot_0.0.1-dev_linux_arm64.tar.gz) | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.1-dev/hubnot_0.0.1-dev_linux_arm64_native-only.tar.gz) |
| macOS 12+ | Intel | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.1-dev/hubnot_0.0.1-dev_darwin_amd64.tar.gz) | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.1-dev/hubnot_0.0.1-dev_darwin_amd64_native-only.tar.gz) |
| macOS 12+ | Apple Silicon | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.1-dev/hubnot_0.0.1-dev_darwin_arm64.tar.gz) | [Download](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.1-dev/hubnot_0.0.1-dev_darwin_arm64_native-only.tar.gz) |

Standard builds include the optional GitHub transfer adapter. Native-only builds
omit that adapter and retain every native feature, including local `hn gh issue`
commands. Both variants install as `hn`; choose one. Protected checks and
automerge remain independently disabled by default.

Download [SHA256SUMS](https://github.com/LynnColeArt/hubnot/releases/download/v0.0.1-dev/SHA256SUMS)
alongside your archive and verify it before extraction. On Linux, use
`sha256sum --ignore-missing -c SHA256SUMS`. On macOS, use
`shasum -a 256 <archive.tar.gz>` and compare the matching checksum entry.
Extract your archive, enter its directory, then install:

```sh
mkdir -p "$HOME/.local/bin"
install -m 0755 hn "$HOME/.local/bin/hn"
export PATH="$HOME/.local/bin:$PATH"
hn version
```

Keep that PATH setting in your shell configuration if needed. The archives also
contain the MIT license, build provenance, and product documentation. See the
[changelog](CHANGELOG.md) for release history and next-release requirements.

Both Linux architectures have been runtime-tested: AMD64 through packaged CLI
smoke checks, and ARM64 through native resource and policy acceptance scenarios.
Both Apple Silicon variants passed packaged CLI smoke checks on macOS; Intel Mac
builds have been cross-compiled and inspected but have not been runtime-tested.
macOS downloads have no Apple Developer ID signature or notarization, so a
quarantined download may require approval in Privacy & Security. The
[release notes](https://github.com/LynnColeArt/hubnot/releases/tag/v0.0.1-dev)
include the detailed platform evidence and distribution limits.

## Build and verify

Building from source requires Go 1.26 or newer and Git 2.x. Docker and Python
are not required for the following build and test commands.

```sh
go build -trimpath -o hn .
go test -count=1 ./...

# Omit GitHub only, or every network adapter. Native commands remain available.
go build -trimpath -tags nogithub -o hn-local .
go build -trimpath -tags noforge -o hn-native .
```

The first release's separate `build-release.py` packaging script requires
Python 3.12+, Git, and Go 1.26.4 to reproduce its archives. That requirement is
limited to that historical packaging script. Current source includes the
Go/Git-only packager:

```sh
go build -trimpath -o hn-release ./cmd/hn-release
./hn-release --rev COMMITTED_REV --version 0.0.2-dev --out NEW_DIRECTORY
```

The output directory must not exist. Optional `--source DIRECTORY`,
`--targets linux/amd64,linux/arm64,darwin/amd64,darwin/arm64` and
`--variants standard,native-only` select declared inputs. The packager isolates
the exact committed source, disables automatic VCS stamping, uses no shell or
module network, and writes normalized binary/source archives, `BUILD.json`,
`PROVENANCE.json` and `SHA256SUMS`. It never tags or publishes. Current
native-only packages exclude all network adapters with `nogithub,noforge`;
the published first release's native-only assets excluded GitHub only.

The packager cross-compiles for macOS and Windows; execution on those packager
hosts remains unverified for this delivery. CLI target cross-compilation also
requires separate runtime evidence. The download/platform statements above
refer to the immutable first release, whose tags and assets stay unchanged.
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
```

Default recall is policy-qualified, active, local-only, and bounded to 20
records and 65,536 encoded content bytes. Every item retains its full IDs,
anchor, lifecycle edges, evidence, applicability, signature, trust class, and
content digest. Memory content and handoff next actions are untrusted inert
data—not instructions, truth, policy authority, or permission to act.

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
hn memory record|handoff|supersede|retract|challenge|show|recall|index
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

The operational-alpha column is implemented. The deferred column describes
the post-alpha roadmap; those capabilities are not implied by alpha completion.

| Area | Operational alpha | Explicitly deferred |
| --- | --- | --- |
| Policy | Show, validate, diff, and govern amendments from exact base bytes; opt-in protected checks and explicit native automerge | Background merge queues and implicit role migration |
| Identity | Distinct actors, mutual device/successor facts, local keyring, retryable planned rotation | Lost-key, compromise, social/organizational recovery, or concurrent writers sharing one actor key |
| Replication | Exact selections, quarantine, positive budgets, validation, atomic accepted refs, compatibility-all | Portable hard pre-download quotas, moderation, selective deletion, and global redaction |
| CI | Repository-defined actions, default Bubblewrap runner, explicit unsafe host fallback | Secrets, configurable network access, strong CPU/memory/disk/process quotas, and portable/container backends |
| Memory | Signed streams, exact anchors/evidence, lifecycle projection, bounded lexical recall, disposable local index, and selected transport | Automatic capture, embeddings, semantic truth, autonomous action, federation, moderation, redaction, retention enforcement, and erasure guarantees |
| Product | CLI and Git-native signed facts | Notifications, general search, web UI, discovery, and a stable protocol compatibility promise |

Published immutable facts may be superseded by new facts, but the alpha cannot
promise global erasure from every replica.

## Documentation

- [Changelog and next-release requirements](CHANGELOG.md)
- [Rich issues](docs/issues-v1.md)
- [Projects and typed items](docs/projects-v1.md)
- [Discussions and pinned answers](docs/discussions-v1.md)
- [Native workflows and explicit execution](docs/workflows-v1.md)
- [Optional forge interoperability and local gh subset](docs/github-interoperability.md)
- [Protocol and storage](docs/protocol-v0.md)
- [Agent Memory Protocol and operator guide](docs/memory-v0.md)
- [Agent memory safety model](docs/memory-safety.md)
- [Identity continuity and keyring safety](docs/identity-v0.md)
- [Governance and policy amendments](docs/governance-v0.md)
- [Selected replication and shallow recovery](docs/replication-v0.md)
- [CI and runner threat model](docs/ci-v0.md)
- [Hosted Git compatibility](docs/host-compatibility.md)
- [Operational self-hosting proof](docs/self-hosting-alpha.md)

## License

Hubnot is licensed under the [MIT License](LICENSE).
