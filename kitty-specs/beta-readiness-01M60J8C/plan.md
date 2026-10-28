# Implementation Plan: Hubnot beta readiness

**Branch**: `feature/beta-readiness` | **Date**: 2026-10-28 | **Spec**: [spec.md](spec.md)

## Summary

Establish a prospective beta contract, then prove released-data compatibility
and public/private recovery before qualifying providers, packages, scale and
external adoption. Keep signed facts and existing wire versions unchanged unless
a concrete compatibility defect requires an explicitly reviewed change.
The 2026-10-28 owner amendment adds unified native contribution/moderation after
the initial recovery work and before boundary review and the beta verdict.

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

## Unified contribution and moderation architecture

Use a provider-free shared contribution model for issues, discussion content,
code proposals and applicable native project/workflow operations. Define native
signed submission/decision records, project identity, exact payload/revision
bindings, and optional operation permissions. Reuse existing identity, signature,
Git storage, quarantine and governance mechanisms where they fit. Introducing
another mandatory service, account, authoritative database or code-review system
is outside scope.

One policy evaluation distinguishes submission, project admission and existing
merge/execution authorization. Permissive issue/reply rules can automatically
admit ordinary contributions; a manual moderator review is not mandatory for
every message. A contributor can never self-enable policy, manufacture another
author's edit, or convert admission into review, CI or acceptance evidence.
Policy enabling and amendments use deliberately selected legitimate project
authority; source-provided policy and imported roles cannot adopt themselves.

Support a bounded contribution artifact/reference handoff and a native pending
queue that can be listed and inspected locally. Wire size, decoded object/event
sizes, count/depth and aggregate pending retention have positive finite limits;
streaming reads and cancellation stop at the declared bounds before admission.
Define an intake operation/window and charge automatic admission to its budget
and to aggregate retained pending/admitted content. Draining queues, hiding data,
or creating actor keys must not silently reset accounting or free retained
history; exhausted budgets refuse intake until deliberate authorized action.
Select limits before contacting a source and account across different actor
keys. Existing Git pack quarantine alone cannot supply hard transfer quotas.
No implicit compatibility-all fetch, arbitrary ref update, workflow execution,
provider discovery or ingress server is introduced. An optional hosted relay
can be added later; the first complete native journey works with local exchange.

Keep original signed payloads intact. Admission/moderation creates signed scoped
facts targeting an exact content revision or actor and evaluated authority. Implement admit/reject,
hide/restore and author block/unblock, deterministic conflict/revocation rules,
and explicit raw inspection distinct from the moderated shared view. Project
admission is distinct from cryptographic transport verification and from signed
proposal acceptance. Moderation cannot guarantee deletion from other replicas.

New pure model/policy/record functionality belongs in `internal/contribution/`
(WP10); bounded staging/exchange in `contribution_exchange.go` (WP11); live CLI,
native permission/view and moderation integration in WP12; public acceptance
and product guide in WP13. WP10 records concrete CLI/wire examples and real
integration seams before downstream implementation. Update ownership through a
reviewed planning amendment if a necessary shared file is not declared; do not
edit WP03's `store.go` or silently broaden a glob. A foundational library is not
claimed as the completed workflow: WP12 wiring and WP13 public proof are required.

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
| IC-10 Native contribution model, policy and signed record foundation | WP10 | FR-015, FR-018, FR-019, FR-020 |
| IC-11 Bounded exchange and pending intake | WP11 | FR-016, FR-017, FR-020, NFR-006 |
| IC-12 Live authorization, admission and moderation views | WP12 | FR-015, FR-018, FR-019, FR-020, C-007 |
| IC-13 Public contribution and abuse-resistance proof | WP13 | FR-021, NFR-006, NFR-007 |

## Delivery sequence

```text
WP01 support contract → WP02 compatibility → WP03 recovery → WP04 interruptions
                                                     ├→ WP05 providers
WP04 → WP10 native foundation → WP11 bounded exchange → WP12 live moderation
                                                     → WP13 public proof
WP13 → WP06 integrated candidate platforms + WP07 integrated runtime limits
WP04 + WP07 + WP13 → WP08 boundary review
WP04 + WP05 + WP06 + WP07 + WP08 + WP13 → WP09 external pilots / beta verdict
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
   Read-only inspection confirmed the same defect in `appendMemoryAtHead`:
   WP03 also owns `memory_store.go` for a same-actor/same-stream guard before
   writes. Compare signed sequence/ID continuity, including known divergence,
   rather than only the greatest sequence. An accepted older prefix or another
   stream must remain independent. Reuse validated read primitives without
   importing audit packs as a read side effect or silently imposing unrelated
   issue-catalog restrictions. Preserve existing publication behavior; a stale
   memory sync can fail its push after successfully importing a newer head.
4. WP04: reuse existing interruption fault seams and add only missing public
   assertions for supported recovery. Confirm retries never infer remote absence.
5. Every Go WP: formatting, `go test -race -count=1 -timeout 20m ./...`, vet,
   build and diff check at a fixed committed head. Avoid simultaneous expensive
   full runs; targeted iterations precede one final full run per reviewed change.
6. WP05–09: explicit observation records for real provider/platform/user work,
   practical measurements and independently reviewed disposition of findings.
   WP06/WP07 wait for WP13 so candidate packages and operating-limit measurements
   include the complete runtime rather than a pre-contribution build.
7. WP10–13: test common operation policy, signed decision integrity and old
   behavior when disabled; use hostile valid signatures, many-key/aggregate
   saturation, oversize/truncation/dependency cases, canceled/replayed intake,
   stale or revoked moderators and conflicting decisions. Exercise an unknown
   contributor denied upstream push, automatic issue/reply admission under
   explicit rules, moderation and independent existing proposal merge gates.
   Run standard/native-only public CLI journeys and observer reconstruction
   with no credentials, external network, daemon or copied signing keys.

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
- Admission policy is about what the project accepts into its scoped view, not
  proof of a unique person or permission to change trusted branches. Signed spam
  and Sybil actors must not evade aggregate intake limits.
- Default-off behavior must preserve old views and operations. Enabling a new
  moderated view must explicitly account for the historical baseline, explain
  pending versus admitted content, and never reinterpret old signed bytes.
- Current selected replication receipts and post-download budgets remain valid
  for their existing purpose; never advertise them as a full public ingress DoS
  boundary. Supported contribution ingress requires independent early bounds.
