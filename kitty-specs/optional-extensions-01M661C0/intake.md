# Confirmed mission intake

Date: 2026-10-30. Mission: optional-extensions-01M661C0.
Owner instruction: package the extension system as a separate mission, including
custom types and the Spec Kitty use case, preserving the default Hubnot workflow
as a hard requirement.

## Discovery already supplied in conversation

| Topic | Confirmed source | Disposition |
|---|---|---|
| Actor and trigger | An external product may sit on Hubnot; ThickTicket is the first requester. | Optional external application host and native adapters. |
| Information model | Owner asked for custom document, list and ticket types. | Versioned namespaced types, records, collections and relationships. |
| Concrete consumer | Spec Kitty doctrine/charter could be implemented as a plugin on Hubnot. | Demonstrate representative data and explicit selection; no doctrine port. |
| Memory reuse | Owner suggested extending memory; discussion identified reusable provenance/history/search. | Generalize suitable primitives, preserve memory wire/authority semantics. |
| Default UX | Owner explicitly does not want to hurt the default use case. | FR-001/NFR-001/002/005 and SC-001/002/010 are hard acceptance. |
| Optionality | Existing owner requirement: native standalone app and independent optional integrations. | No automatic module launch, new required runtime or provider account. |
| Delivery | Final owner message asks for a separate mission. | Planning only; no implementation, release or beta gate amendment. |

## Branch and sequencing decision

Started on feature/beta-readiness. The helper's default target was that branch,
with branch_matches_target=true. To honor separate scope, the canonical mission
create command created feature/optional-extensions and uses it for planning/base
and mission merge target (matching true after create). This routine branch name
is an agent-selected implementation default, not an additional owner quotation.
A later separate PR may target main after the approved beta foundation is integrated.
No beta runtime/history was reset; two generated beta projection files were
preserved and narrowly committed at 14333ba38222398bfd08013870c9f51912ab2e2f.

## Scope boundary

An example consumer demonstrates the general data model. It does not implement
Spec Kitty's resolver, activation graph traversal, enforcement or Java service.
A generic record never acquires prompt/policy authority from a signature or search.
No dependency is added to the base application by these planning artifacts.

## Planning choices

One explicit out-of-process local transport is planned. Generic native data
interfaces do not grant governed policy, runner, merge or identity operations.
The first host advertises unsupported surfaces truthfully. Proposed limits and
mechanisms are engineering defaults recorded in plan/contracts, not observed
performance or implementation results.

## Sources

- Existing ThickTicket user brief and fixed-ref capability inventory, captured
  before planning under /home/lynn/Documents/Codex/2026-10-24/wa/work/optional-extensions-planning-20261030/.
- Spec Kitty current public pack architecture:
  https://github.com/spec-kitty/spec-kitty-open-packs/blob/main/docs/architecture/overview.md
- Current charter service status and ownership:
  https://github.com/spec-kitty/spec-kitty-charter/blob/main/README.md
- Hubnot fixed candidate f8c68bebbaa3845d1eaaac02e3638f30dc062bbd and approved beta lane
  source used for the preceding capability assessment; published alpha remains separate.


## Subsequent execution authorization — 2026-10-30

Owner: "Perfect. Let's get back to the extension mission—break it into work packages and start implementing, with preserving the default Hubnot workflow as the first gate."

Task authoring and implement/review are now authorized. Existing planning-stage
notes remain historical. The first gate imports/rechecks the approved beta runtime,
dispositions the issue23 test assumption and proves the native baseline. Every
feature work package depends on its approval. No release or main push is implied.
