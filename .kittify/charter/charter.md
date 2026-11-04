# Hubnot Project Charter

This is the human-readable companion to the authoritative runtime charter in
`.kittify/charter/charter.yaml`.

## Purpose

Hubnot supplies the collaboration layer Git deliberately does not: portable
issues, proposals, reviews, policy decisions, CI requests and attestations. Its
central promise is that this state travels with a repository, remains signed
and independently verifiable, and does not depend on a particular forge.

## Engineering constraints

- Implement the native `hn` CLI in Go, preferring the standard library and
  narrow, explicit interfaces.
- Keep Git as the content store and transport. Do not introduce a mandatory
  service, service database, GitHub workflow, or Docker dependency.
- Treat all fetched collaboration data and repository-defined execution as
  hostile input. Trust is explicit policy, never ambient authority.
- Signed history is immutable. Corrections and conflict resolutions create
  linked successor events rather than rewriting what participants signed.
- Observable protocol behavior is specified with examples and tested through
  public CLI and repository boundaries, including real temporary remotes.

## Quality gates

Before each work-package review, Go changes pass relevant behavioral race
regressions, the native fast/default controls, formatting checks,
`go vet ./...`, `go build ./...`, and `git diff --check` at the reviewed source.
Before mission acceptance, run native integration/migration and the protocol,
trust, runner, replication, recovery and compatibility cases affected by the
mission. Any observed functional failure blocks review or acceptance.

Keep exhaustive capacity/stress, transport-deadline and real-clock soak tests
as explicit native release qualification jobs. Track unrun selections before
source-only merge; do not claim a scoped pass is an exhaustive pass. Preserve
assertions, fixture sizes and deadlines. Hosted PR checks remain bounded.
This policy was made explicit during runner-efficiency after the aggregate
twenty-minute suite timed out; that failed/incomplete run remains evidence.
Wire-format, trust,
runner-isolation, policy, or recovery changes update the relevant documentation
in the same mission.

Every work package is reviewed before approval. A mission uses a focused branch
and Spec Kitty execution lanes. As Hubnot gains the necessary capabilities,
we also dogfood its signed proposal, review, CI, decision, and merge path; that
demonstrates interoperability, but self-approval is not independent trust.

## Current product boundary

Core collaboration requires only Git. Hardened Linux pipeline execution may
require Bubblewrap. Unsafe host execution remains an explicit per-invocation
opt-in. Correctness, deterministic evidence, and bounded handling of untrusted
data take priority over premature performance optimization.

The first governed mission is proposal revision and conflict recovery: preserve
the original proposal, link it to a new resolved head, and reevaluate all
approval and CI evidence against that exact revision.

## Terminology Canon

`Hubnot` is the product and Go module name. `hn` is the only active executable
and short protocol namespace; active storage uses `.hn/**`, `.git/hn/**`, and
`refs/hn/*`, with `HN_*` environment variables and `hn` wire-version prefixes.
The former short namespace appears only in immutable or explicitly dated
historical evidence and is never an alias or compatibility surface.

Project-owned governance directives use `HN-001` through `HN-005`. External
doctrine identifiers such as `DIRECTIVE_037` retain their upstream names.

## Regression Vigilance

A namespace change must update runtime constants, executable tests, active
configuration, maintained documentation, and charter terminology together.
Before review, search each derived old-token form and classify every survivor
against the mission occurrence map. Frozen signed facts, journals, legacy
configuration, and dated evidence must remain byte-identical.

## Code Review Checklist

Review exact namespace values across code, tests, active configuration, and
current documentation; verify private keys and generated binaries are absent
from the diff; recompute frozen-evidence digests; run formatting, static,
focused, and full tests appropriate to the change; and require every remaining
legacy token to have a specific occurrence-map classification.
