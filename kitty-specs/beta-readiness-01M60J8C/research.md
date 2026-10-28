# Beta-readiness discovery

## Intent and delivery order

The owner authorized the full beta-readiness mission and explicitly ordered the
support contract first, followed by compatibility and recovery. Qualification,
reliability and external-user work remain part of the mission; missing external
evidence must remain visible rather than being inferred from local tests.

## Findings and decisions

1. `README.md` calls the current product an operational alpha, explicitly without
   stable compatibility. Write a prospective beta support contract without
   relabeling the published alpha or promising untested platforms.
2. `event.go` verifies exact signed `hn/0` payloads; identity records, memory,
   policy, archives and migration journals have separate versioned contracts.
   Stabilize those surfaces explicitly, without gratuitously renaming protocol
   versions or reserializing historical facts. Stable fixtures must originate
   from released writers, not the reader under test.
3. `docs/identity-v0.md` distinguishes public continuity from private keys and
   rejects shared actors across devices. Recovery must distinguish keyless
   reconstruction, replacement-device continuity, and exclusive disaster restore
   of an existing signer. Never treat key possession, imports or continuity as
   inherited policy authority.
4. `docs/project-migration.md` excludes keys, local authority and recovery
   journals from public snapshots. A public archive is not a private-state
   backup. Document and rehearse both tracks with ordinary Git and Go tests.
5. `docs/host-compatibility.md` distinguishes custom-ref transport from provider
   APIs. Live adapter evidence and custom-ref hosting evidence must stay separate.
   Local conformance suites alone do not establish live account compatibility.
6. `docs/filesystem-portability.md` and `.github/workflows/portability.yml` already
   provide platform regression and actual-package public journeys. Reuse them;
   do not create duplicate test scaffolds or normalize away OS temp aliases.
7. Published 0.0.3-dev is source `01a152cfbb822a7be9ccbc51dac34e3caa0cf925`.
   The README still targets 0.0.2-dev and the changelog still labels fixes
   unreleased. Update maintained release status while preserving old assets.

## Boundaries

- No new mandatory service, provider, Python runtime, scheduler or database.
- Protected checks, automerge and independent audits stay independently opt-in.
- No shared active signer, social recovery, global erasure or trust inference.
- No claim of live-provider, Intel Mac, exact Mac release-byte or external-user
  qualification without evidence of that particular observation.
- Recovery recipes may use existing Git/Hubnot operations; introduce new product
  commands only when inspection establishes a necessary gap.

## Authorized contribution-workflow amendment

The owner explicitly added unified native contribution and moderation to this
beta mission after discussing public OSS contribution without upstream push
rights. Code and communications share safe intake and operation-specific
authorization. New behavior remains independently optional and disabled.

Current `commands.go` publishes local facts using ordinary Git push; receiving
a contributor's data cannot therefore imply they have upstream push rights.
`docs/replication-v0.md` describes selected quarantine verification and accepted
refs, per-selection budgets and the pre-download quota limitation. No saved
selection means compatibility-all; do not reuse it as a public-inbox default.
`docs/threat-model.md` separates signed attribution from native authority and
records that moderation/global redaction are not existing alpha guarantees.

The new workflow must supply bounded artifact/reference exchange and explicit
pending/admitted states, with policy-governed automatic or manual admission and
signed scoped moderation. It must not advertise existing post-fetch validation
as hard transfer protection. Prefer a bounded supported contribution artifact
over fetching arbitrary untrusted Git packs. A hosted inbox/relay is optional
future convenience; it is neither required for the native acceptance journey nor
implicitly installed by enabling policy.

Implementation details and exact new CLI/wire examples will be specified by the
new foundation WP before downstream integration. Its design must identify the
real reader/writer/replication seams and refresh ownership through the framework
if additional shared surfaces are necessary. This is implementation research,
not permission to defer required behavior or claim unused library code is a
working contribution workflow.

## Open evidence obligations

Real-provider disposable project access, supported Intel Mac execution, download
experience/signing policy, practical size baselines, and external pilot results
must be established in later WPs. They do not block authoring the initial
contract or testing compatibility and local recovery. No beta publication is
authorized merely by completing those initial WPs.
