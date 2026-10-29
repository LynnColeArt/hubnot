# Candidate package qualification

Status: qualification in progress, 2026-10-29. The published `0.0.3-dev` is an
alpha. `0.1.0-beta.1` is an **unpublished qualification candidate**, not a beta
release or tag. The version was selected before freezing source because the
remote tag lookup and release inventory contained no such version; qualifying
these assets can support a later publication decision without rebuilding them.
Nothing in this document publishes or adopts the prospective
[beta support contract](beta-support.md).

## Immutable candidate and evidence

The optional `.github/workflows/portability.yml` dispatch path builds one full
bundle from its committed checkout H using Go **1.26.4** and explicit version
`0.1.0-beta.1`. It creates eight standard/native-only binary tarballs, one source
tarball, `PROVENANCE.json` and `SHA256SUMS`. All four native runtime jobs download
the same artifact. They independently verify every archive checksum, the pinned
provenance digest, source commit/tree, BUILD record, executable digest, compiler,
target, variant and version before running. The actual Go build information must
agree with BUILD; native-only means combined `nogithub,noforge` build tags.

The frozen H and completed observations will be recorded here after the run.
Source tests and a successfully cross-compiled archive are not runtime passes.
An embedded VCS stamp is not the source oracle: the observed checkout, GOMOD,
commit/tree, all Go/module input digests and explicit BUILD bind this candidate.
In particular, the Go SDK may see a primary checkout when a linked worktree has
a `.git` file. Candidate packaging uses `-buildvcs=false`.

Later documentation-only evidence commits may describe H without repackaging;
they must state that all application, module and harness inputs are byte-identical
to H. A runtime change requires a new candidate and requalification. A PR's
synthetic merge candidate qualifies that synthetic commit only.

## Candidate matrix

Each row has two separate variants. None is passed until actual results identify
its exact bytes and observed environment.

| Native package target | Explicit runner pool | Candidate runtime status |
| --- | --- | --- |
| Linux AMD64 | `ubuntu-24.04` | Pending |
| Linux ARM64 | `ubuntu-24.04-arm` | Pending |
| macOS Apple Silicon | `macos-15` | Pending |
| macOS Intel | `macos-15-intel` | Pending |

These mappings were rechecked on 2026-10-29 against the
[official GitHub runner reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners).
The pool name is not an OS/Git/filesystem version guarantee. Runtime observations
record `uname`, OS release, Go/Git versions, filesystem, temporary environment,
source inputs and the macOS translation flag; a translated process refuses the
native qualification path. A latest runner name never proves both architectures.
Windows remains a future track outside this matrix.

The baseline is a full ordinary local checkout, an ordinary local filesystem and
explicit local bare Git transport. Git is required on PATH; the tested Git
versions are observations rather than a claim about every Git 2.x release.
Linked-worktree contribution accounting/locking is exercised by the WP12 journey.
Shallow/partial/promisor configuration and hosted custom-ref transport retain the
separate restrictions in [the support contract](beta-support.md) and
[host compatibility](host-compatibility.md). Case-sensitive/case-insensitive
filesystem claims require actual environment records; hostile case-collision
Git-tree refusal remains source-level packager evidence.

## Installation and runtime cases

`TestBetaPlatformCandidateBundle` independently checks the full transported
bundle. `portableJourneyBinary` verifies each selected native executable; explicit
missing, malformed, wrong-provenance or incompatible packages fail before an
application build. The normal source builder keeps its original behavior when
no package flags are supplied. `TestBetaPlatformInvalidPackageNeverBuilds` puts
a trap compiler on PATH to detect accidental source fallback.

The selected package journeys cover:

- Copy the verified executable into a disposable user bin directory, preserve
  its digest, find `hn` on PATH and check the explicit candidate version.
- Run native collaboration, opt-in governance, independent audits, archive/local
  migration, ordinary Git transport and native-only adapter exclusion with Git
  alone on the child PATH. No Go, Python, provider CLI, provider account or
  service is a runtime dependency.
- Read public fixtures written by `0.0.1-dev`, `0.0.2-dev` and `0.0.3-dev`, retain
  signed bytes/IDs/refs, and check supported continuation/unsupported-format refusal.
- Rehearse public keyless reconstruction, exclusive private signer restoration,
  stale-root refusal, and public rotation/migration journal refusal. Internal-only
  transfer-journal instrumentation remains source proof, not package proof.
- Exercise WP13 public contribution, scoped moderation, authority separation and
  reconstruction in both variants. WP12 public intake/retry/linked-worktree
  journeys additionally receive the exact selected executable through the
  `HN_CONTRIBUTION_TEST_BINARY` plus `_SHA256` hook; outer Go test tags do not
  select a child binary. All nine provider-independent WP12 journeys run in
  both variants. The raw hidden-workflow publication witness explicitly selects
  the standard provider binary and runs only in the standard WP12 batch.
- Execute actual packaged `memory record` and regex `memory grep`, check match
  bytes/warning and invalid-pattern refusal. The source `TestMemoryGrepCLI`
  builds its own executable and is separately classified.
- On macOS, use literal `/tmp` and `/var/tmp` session/archive paths and a closed
  child environment with **no TMPDIR**, import foreign records locally and export
  them again. No physical-path/TMPDIR workaround is injected.

The unshipped `nogithub`-only variant is intentionally excluded in explicit
package mode. Standard and native-only both execute; this exclusion must never
hide a missing shipped asset. Source PR checks retain the original three variants.
Go tests and their race instrumentation run on the host; release applications are
ordinary non-race binaries. A cross-compiled test harness on an authorized ARM
host without Go is non-race evidence and must be labelled as such.

A reproducible explicit package command is:

```sh
go test -race -count=1 -timeout=5m -v -run '^TestBetaPlatformCandidateBundle$' . -args -hn-package-dir UNPACKED -hn-package-bundle BUNDLE -hn-package-version 0.1.0-beta.1 -hn-package-compiler go1.26.4
```

Use the complete runtime regex in the workflow for integration, compatibility,
recovery, journals and contribution coverage. Run from H's checkout. Preserve
raw commands, start/end times, exits, logs and original archive/binary hashes.
Private fixture keys remain disposable and are never uploaded; process logs
contain public data and fixture-only credentials, not operator secrets.

## Qualification fixture controls

The package install/memory/filesystem preflight runs before the longer journeys.
It uses an explicit per-command fixture Git author, without configuring an
operator's global identity. Public subprocess transcripts identify the selected
program, working directory, arguments, exit and output; each invocation writes
its own evidence file.

Released-writer refusal cases compare the complete repository file snapshot,
including signed history and private state. Their disposable clones and import
destinations explicitly disable automatic Git maintenance before the measured
refusals. A clone also receives those settings for that command so background
maintenance cannot start before local fixture configuration is written. This
is a test fixture control, not a Hubnot runtime setting or weaker snapshot
comparison. Failure diagnostics report changed paths without disclosing private
file contents. The Linux qualification additionally repeats the actual-package
compatibility cases three times to check the transient-file failure observed
with Git 2.56.0; repeated cases are separate from the one-pass integrated batch.
Their successful completion remains an observation to record, not an assumption.

## Download experience and signing decision

CI artifact extraction and native execution do not exercise browser quarantine,
Finder launch, Gatekeeper prompts or an independent user's download/install
journey. The candidate has no Apple Developer ID signing or notarization.
Manual browser downloads of these exact archives on both Mac architectures and
human adopter feedback remain pending even after a green CI matrix.

Recommendation: qualify the unsigned bytes now and decide Developer ID signing
and notarization before presenting a broadly downloadable beta as a smooth Mac
installation experience. That decision needs the owner's Apple Developer account,
chosen signing identity, protected CI credential handling, notarization service
access, intended distribution channel and retention/update procedure. Signing
changes executable/archive bytes, so signed assets need new digests and the
relevant runtime/download checks. Do not disable host protection globally.
An owner may deliberately distribute unsigned assets with clear documented prompts;
that separate choice must not be inferred from successful CI execution.

The previously authorized Linux ARM host Golden Goddess can supply additional
exact-byte observations after known-host/reachability/native checks. Pink Menace
name resolution previously succeeded but SSH port 22 refused; there is no remote
Mac observation to transfer. Missing browser, hardware or user cases remain
pending rather than narrowing the support promise silently.
