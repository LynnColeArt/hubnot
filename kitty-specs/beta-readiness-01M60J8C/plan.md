# Implementation Plan: Hubnot beta readiness

**Branch**: `feature/beta-readiness` | **Date**: 2026-10-28 | **Spec**: [spec.md](spec.md)

## Summary

Establish a prospective beta contract, then prove released-data compatibility
and public/private recovery before qualifying providers, packages, scale and
external adoption. Keep signed facts and existing wire versions unchanged unless
a concrete compatibility defect requires an explicitly reviewed change.

## Technical Context

- Go 1.26+, standard library and ordinary Git; one existing CLI/module.
- Git stores code, signed collaboration and memory; private identity, audit
  authority, selections and journals remain local and explicitly separate.
- Supported environment claims follow actual qualification. Linux and macOS
  AMD64/ARM64 are candidate targets, not four already-proven beta platforms.
- Existing resource, archive, issue JSON and migration contracts remain canonical.
- Existing public subprocess tests, Go-only packager and portability CI are
  extended rather than replaced. Use normal temp environments on macOS.
- Workload thresholds must come from measured practical scenarios, not invented
  promises or microbenchmarks used as release gates.

## Charter Check

The mission preserves HN-001 native independence, HN-002 exact evidence, HN-003
least authority, HN-004 immutable recovery and HN-005 documentation sync. No new
runtime dependency or mandatory provider is needed. Every WP receives an
independent review; Go changes satisfy the full charter gates. Private test
keys stay ephemeral; never commit credentials or existing user keyrings.

## Architecture and ownership

`docs/beta-support.md` owns the prospective support/evolution promise and links
existing detailed format contracts. `docs/recovery.md` owns supported recovery
procedures and exclusions. These are maintained product documents; the mission
stores plans and evidence, not parallel copies of product specifications.

`testdata/compatibility/` holds small released-writer public-only fixtures,
provenance and expected public observations. Its corpus is immutable evidence;
fixture generation is an explicit developer action, never a test startup step.
The test runner invokes the current compiled public CLI, reads retained bytes,
compares the history and checks continuation/refusal. It must not reconstruct
fixtures with current internal encoders or pin harmless human output formatting.

`beta_compatibility_test.go` and `beta_recovery_test.go` own public boundary
rehearsals. Reuse existing compiled test helpers when they fit; otherwise create
one narrow subprocess helper for closed environments and repository snapshots.
Before adding another harness, inspect `migration_acceptance_test.go` and
operational tests. No production backup command is assumed necessary: existing
Git transport plus protected offline private backup may satisfy recovery if the
public procedure actually passes. Repair real defects where tests expose them.

Public backup uses an explicit complete selection of code and collaboration/
memory refs and their reachable objects. A plain clone or a selected project
snapshot is not automatically a complete backup. Public restoration validates
facts and admits selected remote history without adopting policy authority.

Private disaster recovery is a distinct offline operator procedure for an
exclusively controlled signer, not key cloning to a second active device.
Fencing the old writer, reconciling the latest actor chain and handling stale
journals are prerequisites. Audit authority is deliberately re-adopted after
verification; migration/transfer journals must not be moved and replayed against
a different destination by assumption. If keys and all policy authority are
lost, the guide must explain the existing unrecoverable boundary.

## Implementation Concern Map

| Concern | Owner | Requirements |
| --- | --- | --- |
| IC-01 Support promise/current release docs | WP01 | FR-001, FR-002, FR-014 |
| IC-02 Released writer/read compatibility | WP02 | FR-002, FR-003, FR-004 |
| IC-03 Public/private recovery | WP03 | FR-005, FR-006, FR-007 |
| IC-04 Journal and interruption recovery | WP04 | FR-002, FR-008 |
| IC-05 Real provider journeys | WP05 | FR-009 |
| IC-06 Package/platform distribution | WP06 | FR-010 |
| IC-07 Practical workload limits | WP07 | FR-011 |
| IC-08 Boundary review and fixes | WP08 | FR-012 |
| IC-09 External pilots/final verdict | WP09 | FR-013, FR-014 |

## Delivery sequence

```text
WP01 support contract → WP02 compatibility → WP03 recovery → WP04 interruptions
                                                     ├→ WP05 providers
                                                     ├→ WP06 platforms
                                                     └→ WP07 limits → WP08 boundary review
WP04 + WP05 + WP06 + WP07 + WP08 → WP09 external pilots / beta verdict
```

External evidence dependencies remain recorded. Complete independently
executable local work while waiting; do not mark a credential-dependent live
case complete because its fake-server conformance passed. No final beta tag,
release publication, issue comments or messages to other people are part of the
initial documentation/implementation authorization.

## Verification strategy

1. WP01: validate docs against actual commands, format contracts and release
   metadata; check all current download links and preserve historic sections.
2. WP02: run released-writer corpora offline in standard/native-only builds;
   check exact bytes/IDs/refs, documented semantic JSON and unsupported-input
   nonmutation. Store fixture provenance and a regeneration procedure.
3. WP03: run executable public/private recovery recipes through Git and the CLI;
   compare public history before/after, keyless absence, exclusive signer
   continuation, new-actor nonauthority and unsafe/stale recovery boundaries.
   Recovery inspection found that `store.go` can append from a stale local
   actor ref despite a newer accepted same-actor history. WP03 owns a common
   append refusal guard for known newer/divergent accepted history, with a
   public no-side-effect regression and successful continuation after explicit
   verified expected-old ownership restoration. This is not detection of unknown
   disconnected writers and does not automatically adopt a remote signing head.
4. WP04: reuse existing interruption fault seams and add only missing public
   assertions for supported recovery. Confirm retries never infer remote absence.
5. Every Go WP: formatting, `go test -race -count=1 -timeout 20m ./...`, vet,
   build and diff check at a fixed committed head. Avoid simultaneous expensive
   full runs; targeted iterations precede one final full run per reviewed change.
6. WP05–09: explicit observation records for real provider/platform/user work,
   practical measurements and independently reviewed disposition of findings.

## Risks and decisions

- Current alpha data has not had a stable promise. Declare a tested historical
  baseline and a prospective beta evolution policy, not retroactive support for
  unknown data or every alpha journal.
- Key backup can accidentally create concurrent writers. Emphasize exclusive
  disaster restoration and fresh actors for normal additional devices.
- Provider credentials and Intel Mac access may require owner assistance. Keep
  those actual observations pending and supply concrete test briefs.
- Versioned journals carry in-flight external intent. The contract may require
  finishing a session with its originating compatible binary before upgrading;
  unsupported journal versions must never be treated as new empty sessions.
- A wider test corpus should protect behavior, not become brittle SHAs of
  current generated output or a ratchet against legitimate feature additions.
- Hosted Spec Kitty sync currently reports a locked/unavailable store; local
  canonical mission artifacts and event logs remain the execution record.
