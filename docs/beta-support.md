# Beta support contract

**Status: prospective, 2026-10-28. Hubnot 0.0.3-dev is an alpha prerelease.**
This contract defines the support promise to qualify before the first beta.
It takes effect when a beta release explicitly adopts it and publishes its
passing support matrix. Writing this document does not qualify a beta or change
the guarantees of earlier alpha releases.

**Audience:** Git users adopting Hubnot for project collaboration, and tool
authors integrating with its public CLI. The goal is to make installation,
data retention, upgrades and recovery predictable within a declared scope.

## Native operation and optional features

Prebuilt Hubnot applications require Git on PATH. Source builds and release
packaging require Go and Git. Native collaboration requires no Python, provider
account, hosted service, database or Docker. An ordinary Git remote can carry
code and explicitly selected collaboration refs; a local bare remote is enough.

Standard packages include optional network provider adapters. Native-only
packages omit all of them and retain native collaboration, local/archive
migration, foreign-record inspection, audits and the local `hn gh issue` subset.
That subset is documented command compatibility, not an HTTP service for the
unmodified GitHub `gh` binary. See [provider interoperability](github-interoperability.md).

Protected checks, automerge and independent audits remain separate opt-ins,
disabled by default. Installation, initialization, imports, upgrades and merges
do not enable them or start a worker. CI execution is explicit: the Linux
sandbox may require Bubblewrap; unsafe host execution requires its existing
per-invocation opt-in. macOS has no bundled safe execution backend. These
boundaries are detailed in [governance](governance-v0.md), [CI](ci-v0.md) and
[audits](audits-v1.md).

## Candidate platforms and Git configurations

The beta targets Linux and macOS on AMD64 and ARM64, with standard and native-only
packages. Each supported beta combination must identify the tested OS release,
architecture, Git version, filesystem and actual package. The current statement
that prebuilt alpha applications need Git 2.x is a dependency description, not
a promise that every Git 2.x release or host configuration is qualified.

| Candidate combination | Current 0.0.3-dev observation | Evidence still needed for beta |
| --- | --- | --- |
| Linux AMD64, both variants | Exact release packages passed smoke checks and the integrated public journey | Candidate beta packages and declared environment matrix |
| Linux ARM64, both variants | Exact release packages passed runtime smoke checks | Candidate beta packages and declared environment matrix |
| macOS Apple Silicon, both variants | Native rebuilds from the release source with Go 1.26.6 passed the integrated journey without path/TMPDIR workarounds | Execution of the downloadable Go 1.26.4 release binaries remains pending; candidate beta package/install qualification |
| macOS Intel, both variants | Release archives, architecture, checksums and provenance inspected; binaries not executed | Intel runtime and candidate beta package/install qualification |

These observations come from the [0.0.3-dev release notes](https://github.com/LynnColeArt/hubnot/releases/tag/v0.0.3-dev).
They do not transfer automatically to a new candidate. Windows distribution is
outside this beta target set. Apple signing/notarization is a distribution
decision still to resolve; current downloads have neither.

An ordinary full local checkout and local bare Git transport form the baseline.
Shallow repositories require complete selected dependencies or the explicit
[shallow recovery path](replication-v0.md); an incomplete history is not silently
treated as absent. Automerge and release packaging refuse active partial/promisor
repositories. Linked-worktree, filesystem and hosted-remote claims must name
the exercised operation rather than imply support for every command. Filesystem
behavior follows the [portability contract](filesystem-portability.md).

Provider API migration and hosting `refs/hn/*` are separate capabilities. A host
accepting branch pushes does not prove custom-ref transport or retention.
Ordinary clone omits custom refs; operators must deliberately retain and fetch
collaboration and memory histories. See [host compatibility](host-compatibility.md).

## Data and interface evolution

The prospective beta baseline is the documented native data written by
0.0.1-dev, 0.0.2-dev and 0.0.3-dev, plus project snapshots/archives introduced
in 0.0.2-dev. This baseline must pass released-writer compatibility tests before
it becomes a beta promise. It excludes the retired `nh` namespace and arbitrary
undocumented alpha files. The format documents below remain authoritative for
individual fields and validation limits.

| Surface | Beta evolution rule | Detailed contract |
| --- | --- | --- |
| Signed collaboration facts (`hn/0`) and resource payloads | Newer beta readers retain support for the declared baseline; payloads, signatures, IDs, actor chains and historical meaning stay exact. Corrections append successor facts. | [Protocol](protocol-v0.md), [issues](issues-v1.md), [resources](resources-v1.md) |
| Signed memory (`hn-memory/0`) | Preserve exact envelopes, content, stream ownership and lifecycle edges. The local lexical index is disposable and may be rebuilt. | [Memory](memory-v0.md), [memory safety](memory-safety.md) |
| Policy, pipelines and audit authority | Retain supported version semantics; no implicit role migration, opt-in activation or authority adoption on upgrade/import. Historical policy bytes keep their original meaning. | [Governance](governance-v0.md), [CI](ci-v0.md), [audits](audits-v1.md) |
| Public snapshots and archives (`hn.project-snapshot/1`, `hn.project-archive/1`) | Preserve supported manifests, selected Git objects and retrieved bytes. New incompatible formats need a distinct version and an explicit transition; preservation does not imply provider recreation. | [Project migration](project-migration.md) |
| Saved plans and private recovery journals | Never reinterpret an unsupported version as a fresh session or discard uncertainty. An upgrade must support the session explicitly or instruct the operator to finish/recover it with its originating compatible binary. No blanket cross-version alpha-journal promise. | [Migration engine](migration-engine.md), [migration CLI](migration-cli.md), [transfer recovery](github-interoperability.md#private-recovery-and-publication) |
| Public CLI JSON | Preserve documented field meanings, types and success/error semantics within the declared schema. Additive output fields are allowed; consumers should ignore unknown fields and handle new error codes. Incompatible changes require an explicit interface/schema transition. | [Issue envelopes](issues-v1.md#machine-envelope-and-navigation), [resource JSON](resources-v1.md), [local gh JSON](github-interoperability.md#compatibility-json) |
| Human output, opaque cursors and private caches | Human text is for people, not a parsing API. Cursors belong to their documented query/snapshot; restart pagination after an upgrade. Caches are not backups or authorities. | The individual command guides |

Product release numbers, signed wire versions and CLI schemas are independent.
A new product version does not require rewriting stored data or renaming wire
versions. Introducing a new event kind or format may require newer readers;
older binaries are allowed to refuse it clearly. Do not roll back a binary and
assume it can read facts published by a newer writer. Retain a compatible reader
and a verified backup before upgrading. A security fix may reject previously
accepted invalid data; its release notes must identify the affected boundary and
recovery action rather than silently claim full compatibility.

## Recovery promise and authority limits

Beta qualification must rehearse two distinct recoveries:

- **Public project recovery:** retain code and the selected collaboration/memory
  refs with their complete reachable objects, then reconstruct and verify them
  without a private signer. A plain clone or a scoped export is not automatically
  a complete backup. Public project archives deliberately exclude private keys,
  credentials, journals and adopted audit authority.
- **Private disaster recovery:** restore a protected signer backup only after
  fencing the former writer and reconciling the latest available actor chain
  and any continuing memory-stream roots. Restoring a key and running `hn sync`
  alone does not restore the local signing position. The pending recovery work
  must refuse an append from a stale/divergent local actor head when conflicting
  accepted history is already known locally, without silently adopting it.

Normal additional devices use distinct actors. The client cannot discover an
unknown disconnected writer or guarantee the global latest history. No backup,
import, device relationship or key restoration grants policy roles. Audit
authority must be deliberately adopted after verifying the intended branch.
Without a usable key backup or remaining authorized actors, loss of sole policy
authority has no automatic recovery. Social recovery, shared concurrent signers,
global deletion and erasure from other replicas remain outside this contract.
See [identity continuity](identity-v0.md) and the [threat model](threat-model.md).

## Provider support and practical limits

Installed adapters are experimental until their exact API profile, resource,
direction, permissions and live-account journey are qualified for beta.
GitHub, GitLab, Forgejo/Codeberg, Gitea, Bitbucket Cloud and Azure DevOps have
different capabilities; the [provider guide](github-interoperability.md) records
their implemented profiles and explicit unavailable mappings. Fake-server
conformance is useful evidence but is not a live-provider qualification.

Every supported migration must show selected scope, retrieved bytes, mappings
and losses before applying. Unsupported destination recreation must be preserved,
explicitly mapped or refused; it cannot be reported as lossless success. Foreign
workflows, reviews and users remain inert attributed content and grant no native
execution or approval authority. Timeouts and negative lookups do not authorize
repeating uncertain remote creates.

Existing per-record, pagination and replication budgets remain enforced. Beta
must additionally publish measured practical history/repository envelopes and
recovery costs. Until those measurements exist, no unlimited scale, universal
latency target or portable hard pre-download quota is promised. The runner is
not a hardened hostile multi-tenant execution boundary.

## Qualification and changes to this contract

The beta decision must trace each supported surface to evidence for:

1. Released-writer compatibility, unsupported-format refusal and upgrade safety.
2. Keyless public reconstruction, exclusive signer restoration and interrupted
   operation recovery without historical rewrites or inferred authority.
3. Actual candidate package installation/runtime on each claimed environment.
4. Live provider journeys for each claimed adapter/profile/resource/direction.
5. Practical workload measurements and focused security/reliability review.
6. Independent adopter journeys through installation, collaboration, migration,
   upgrade and restoration, with release-blocking findings resolved.

Unexecuted cases remain pending. A future beta release must identify this
contract, its supported matrix and known limitations; it cannot silently narrow
the promise to make a gate pass. Later changes to supported data or interfaces
need documented rationale, explicit compatibility/migration consequences and
review. This contract requires no mandatory hosted checks, default policy gates,
background service or perpetual pinning of ordinary project rules.
