# Candidate package qualification

Status: qualification in progress, 2026-10-29. The published `0.0.3-dev` is an
alpha. `0.1.0-beta.1` is an **unpublished qualification candidate**, not a beta
release or tag. The contribution promotion context gap was repaired at `5bedb38c`;
a freshly built candidate passed both Linux rows. Mac qualification is still
incomplete: the Intel WP13 group reached the unchanged per-command deadline,
and remaining Mac groups are still running. Earlier H3 results remain historical
observations of different executable bytes. The version was selected before freezing source because the
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

The current repaired source is `5bedb38c2bf8f59938515114297aeedeb959e53f`, tree
`66416a0b8b7b79e663099bc808289533626fe96b`.
[Build and native runtime run 40935607370](https://github.com/LynnColeArt/hubnot/actions/runs/40935607370)
produced [candidate artifact 12925340544](https://github.com/LynnColeArt/hubnot/actions/runs/40935607370/artifacts/12925340544).
Its ZIP SHA-256 is `e40d154999cf500d4bb3aa043a9ee72b111942aac58eb4452ec3b83607bd2dac`;
`PROVENANCE.json` SHA-256 is `a1e4ca916aa26fdc875fa5a7ee7467db8351ab2b717d9c9c109efe865c155578`.
The uncached source race suite passed at this exact source in 1,023.759 driver
seconds, with all 218 Go/module inputs unchanged; vet, standard/native-only
builds, formatting and diff checks also passed. Source-assisted deadline fault
controls remain separate from native package observations. All eight repaired
executables differ from H3, so no previous runtime result transfers to them.
The current documentation/workflow forward retains all 218 Go/module inputs and
`docs/contributions.md` byte-for-byte from this candidate.

The historical H3 source is `9a950e8fbd3acb06f1aaee26fc11021f2a0dbb02`, tree
`bdec63a0e25b72431658e4985d08c0fb87be0aed`.
[Build and native runtime run 40926302591](https://github.com/LynnColeArt/hubnot/actions/runs/40926302591)
produced [candidate artifact 12920397242](https://github.com/LynnColeArt/hubnot/actions/runs/40926302591/artifacts/12920397242).
Its ZIP SHA-256 is `6b24705b20a520d708af39d3bf7238a071cae2746a0cbb95672f9f3148799da1`;
`PROVENANCE.json` SHA-256 is `4f951fed5318ad982b6117ddc9ec7ab9bc815d5ad286de84947c2471bbb2470c`.
The source race suite passed uncached at H3 in 1,036.710 seconds, with all 217
Go/module inputs unchanged; standard/native-only vet and build checks passed.
Those source checks and successful cross-compilation remain separate from
actual native package runtime observations.
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
| Linux AMD64 | `ubuntu-24.04` | Repaired `5bedb38c` standard/native-only: all required groups passed |
| Linux ARM64 | `ubuntu-24.04-arm` | Repaired `5bedb38c` standard/native-only: all required groups passed |
| macOS Apple Silicon | `macos-15` | Repaired candidate preflight and both WP13 variants passed; remaining groups pending |
| macOS Intel | `macos-15-intel` | Repaired candidate preflight passed; WP13 failed at the per-command deadline; unqualified |

These mappings were rechecked on 2026-10-29 against the
[official GitHub runner reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners).
The observed H3 Linux hosts ran Ubuntu 24.04 with Git 2.56.0; their filesystem
reported `ext2/ext3` and outer TMPDIR was unset. Both Mac hosts ran macOS 15.7.9
(build 24G830), Git 2.55.0 and APFS, with their normal `/var/folders/.../T/`
outer temporary directories. Apple Silicon reported `sysctl.proc_translated=0`;
the Intel translation flag was unavailable and its native target/uname agreed.
Child package journeys used their declared closed environments.
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

## Reuse and supplemental runtime execution

The optional workflow retains `candidate_mode=build` and all four native
platforms as its dispatch defaults. Explicit `candidate_mode=reuse` requires
an exact source commit, originating run ID, candidate artifact ID and expected
provenance digest. It validates that the artifact and successful build belong
to the same source and this repository, checks the original ZIP/provenance/
archive digests, and refuses missing, expired or mismatched inputs. It does not
run the packager or compile a replacement app. The runtime jobs check out the
candidate's original source/harness and record the executing workflow revision
separately. Same-repository cross-run downloads use only the ephemeral token's
read permissions; no additional credential is required.

`platform_scope=macos` selects the existing Intel/Apple Silicon rows when only
supplemental Mac observations remain. Such a run cannot claim it executed
Linux; the final matrix must link the separately bound Linux results. The
[download action inputs](https://github.com/actions/download-artifact/blob/v4/README.md#inputs)
and [artifact metadata](https://docs.github.com/en/rest/actions/artifacts#get-an-artifact)
describe this optional CI transport, which is not a Hubnot runtime dependency.

WP13 public contribution journeys have a separate 45-minute aggregate test
invocation; remaining integrated/compatibility/recovery/migration journeys
retain a 30-minute invocation. WP12's two variants each have a separate
20-minute aggregate invocation. A native job allows 90 minutes for these
sequential groups. These are qualification-harness budgets: all existing
per-command deadlines, production limits, assertions and shipped variants
remain unchanged. Independent groups run only after successful package
preflight; a failed required group still fails the job and is never hidden by
later successful evidence collection.

H3's Mac ARM run passed compatibility in 113.51 seconds and both WP13 variants
in 1,471.23 seconds, then exhausted the combined 30-minute invocation in a later
portable journey. No individual command or completed semantic case failed in
that run, but unfinished portable work and WP12 remain unqualified. Intel passed
compatibility in 359.28 seconds, then hit the 45-second subprocess deadline for
standard proposal intake and native-only issue restoration before the global
30-minute alarm interrupted its remaining WP13 cases. These per-command
observations are separate from the aggregate budget; their internal phase and
cause remain unproven. The contribution deadline-contract review confirmed
missing context propagation in public-file promotion seams, but did not
attribute the Intel results to those seams. No larger job budget or reused
artifact establishes that those Intel cases passed. A runtime repair requires
new package bytes and fresh qualification; this reuse path cannot substitute
H3 evidence for that future candidate. No supplemental reuse run has been
claimed here.

The two complete H3 Linux runs each retain 4,018 process transcripts. Exact
required top-level names/counts were checked: two preflight cases, three full
compatibility repetitions, nine one-pass journey cases, ten standard WP12
cases and nine native-only WP12 cases. All exits were zero and source inputs
were unchanged. The only skipped subcases were the two deliberately unshipped
`nogithub`-only selections; standard and shipped native-only both ran.

## Repaired candidate observations and bounded diagnostics

Both complete `5bedb38c` Linux rows retain 4,018 process transcripts each and all
218 source inputs remained unchanged. Each ran two preflight cases, three full
compatibility repetitions, one WP13 group, eight other integrated journey groups,
ten standard WP12 cases and nine native-only WP12 cases. All required exits were
zero; only the two deliberate unshipped `nogithub` subcases were skipped.
An independent supplemental Linux AMD64 operating-envelope recheck used these
same standard/native-only packages with an explicitly bound external WP07
harness overlay. Both 24-unit, four-scenario checks passed in 68.940 and 67.520
seconds respectively. These are observed operating-envelope scenarios, not a
cross-platform latency guarantee.

The repaired Intel WP13 step failed in 1,506.68 seconds. Preliminary live step
logs identify a killed standard `proposal-intake` receive and a killed native-only
`block-author` moderation command at the existing 45-second subprocess context.
Other WP13 groups completed successfully in both variants. Final job artifacts
must corroborate those live observations before closeout. The internal failing
phase, last durable state and cause remain unknown. The finite request/stage
contexts added by the repair do not constitute one cumulative whole-CLI timer.
Apple Silicon's repaired WP13 group passed; its remaining required groups and
both Mac final job outcomes are still pending.

An explicit `diagnostic_mode=intel-standard-45` path is prepared for one standard
happy-journey observation using only this exact source, origin run, artifact and
provenance. It requires reuse mode and cannot build a replacement application.
It checks out the original candidate harness and applies an external observation
overlay; every semantic assertion and the 45-second subprocess context remain.
The independent native qualification jobs are skipped in this mode. Diagnostic
records and artifact names explicitly say `qualification_eligible=false`, so a
diagnostic success never qualifies the platform. No diagnostic dispatch has yet
been authorized or executed.

The overlay records per-process UTC timing, monotonic elapsed duration, PID,
arguments, executable digest, exit/context and output length/hash. Only the
target `proposal-intake` receives before/after observations of the known fixture
journal. Reads refuse unsafe/nonregular paths and inputs over 1 MiB; typed
filtering excludes descriptors, checksums/HMAC material, private paths, arbitrary
failure text and manifest material. A filtered snapshot is capped at 60 KiB,
each output record at 64 KiB and all records at 4 MiB. Raw application/test logs
remain ephemeral and are neither echoed nor uploaded. No additional Hubnot
observer command, Git trace, policy change or longer capture mode is enabled.
Journal observations have separate duration records and may warm caches.
Start metadata writes occur during the command interval, so their overhead is
included in command elapsed time and can affect the observed result. A durable journal status identifies the last recorded phase,
not an exact internal function or the cause of a historical failure.

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
