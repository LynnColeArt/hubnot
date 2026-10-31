# Extension research and design decisions

Date: 2026-10-30. All implementation surfaces below are proposed, not shipped.

## R-01 — Existing capability inventory

Decision: adapt supported native issue/project/item/discussion/reply/workflow-definition
operations; do not create an alternate native writer.
Evidence: existing root main.go/localCommands and resource_engine.go/NativeResourceCommands
are compiled registration seams, not installed external hosts. The fixed-ref inventory
in intake.md enumerates actual replay support and its gaps. Current memory protocol
has six fixed kinds and actor-owned correction streams; it is not a generic shared
document/type host.
Rationale: reuse authority, signatures, exact parents, operation lookup and governed views.
Rejected: arbitrary CLI argv execution or pretending every command is replay-compatible.

## R-02 — Additive typed artifact protocol

Decision: use an independent explicitly versioned custom record model/ref family with
generic inspection/search/export available without specialized executable registration.
Reuse existing identity, safe Git and bound-checking patterns after source inspection,
without replacing hn/0 or hn-memory/0.
Rationale: custom types and multi-author documents are broader than cognition records;
old readers reject unknown native event kinds. Keeping separate histories protects
native compatibility. Memory remains a consumer of its existing contract.
Rejected: add arbitrary kinds to existing signed memory/collaboration chains; stuff
all domain state into untyped decision strings; load a plugin just to verify transport.
Proof required: goldens unchanged, unknown data does not block native paths, independent
clone reconstructs and plugin removal preserves generic access.

## R-03 — Scope and language-independent host

Decision: one local explicitly invoked out-of-process stdio protocol, local installation
and per-physical-repository grants; Go standard library implementation.
Rationale: deterministic local contract with no mandatory daemon/MCP/model/provider.
Reference consumers are standalone executables. The application may have optional
dependencies; base Hubnot packages acquire none.
Rejected: in-process arbitrary code, multiple transports, marketplace, blanket shell bridge.
Threat boundary: same-user modules can access files directly; grants mediate host calls.
Adversarial dependency review: deferred with rationale because no new dependency is
chosen or installed during planning. Host/protocol threat review remains required before
implementation acceptance; dependency changes must reopen this decision and supply
current primary-source/supply-chain review. This is not a completed squad review.

## R-04 — Spec Kitty use case grounded in current public sources

Official sources inspected 2026-10-30:
- https://github.com/spec-kitty/spec-kitty-open-packs/blob/main/README.md
  observed blob f92886dd0368ae0f31bbf6b599618f84f3292e1e.
- https://github.com/spec-kitty/spec-kitty-open-packs/blob/main/docs/architecture/overview.md
  observed blob 4f362546f83c81ce955f49fdd32f0327e1b05ef0.
- https://github.com/spec-kitty/spec-kitty-charter/blob/main/README.md
  observed blob d9bf319d10ed4d318f43bb67b1213fd7e158e9f1.
- Representative directive:
  https://github.com/spec-kitty/spec-kitty-open-packs/blob/main/packs/built-in/directives/001-architectural-integrity-standard.directive.yaml
  observed blob 107c9b26224108b41c0ca02af22f7a470acbf399.

The public model separates offered artifact packs, typed graph relationships,
project activation and action-specific context. The charter repository describes
a Java-service bootstrap while Python remains its reference in core; this is not
a completed Java cutover. Local Priivacy-ai paths reflect an older layout; three
old source-file lookups on current core default branch returned404. They are not
evidence that current doctrine source exists at those locations.

Decision: representative generic records/relationships/collections plus an explicit
exact-revision selection are the Hubnot requirement. Full doctrine interpretation,
layer composition and execution stay outside Hubnot. Examples carry their own
namespace and are not advertised as upstream schema/API conformance.
Rejected: add built-in directive/tactic/charter kinds to Hubnot or require its service.

## R-05 — Default experience is a hard invariant

Decision: registry access and module startup only on explicit extension commands.
Preserve native commands, setup, golden data and dependencies; run failure/disabled/
unknown-data comparisons and paired metrics, with negative controls proving forbidden
effects would be detected.
Rationale: explicit owner instruction. This condition cannot be traded for plugin
features or postponed to documentation. Thresholds in NFR-002 are initial engineering
budgets, not observed data or public performance guarantees.
Rejected: eager scanning/probing on startup; weaken old refusal tests; treat a missing
plugin as a reason native ticketing cannot work.

## R-06 — Capability and authority limits

Decision: first host supports reliable native data families plus new artifact
operations and accepted-change capture. List memory mutations, proposals, policy,
runner/execution, review/accept/merge, identity lifecycle and automatic transport
apply as unsupported until separately contracted.
Native contribution-enabled operations retain their existing admission/visibility
paths. Custom writes with selected contribution policy are unsupported initially,
with a refusal before effects. Do not infer permission from imported schema metadata.
Rationale: missing actor/operation recovery and custom admission profiles are real
gaps, not wrappers to hand-wave away.

## R-07 — Workflow and delivery

Decision: separate mission target feature/optional-extensions, planning-only now,
implementation prerequisite is the approved integrated beta foundation.
Preserve the beta pilot and issue23's independent disposition. No source pass,
exact-package pass or beta adoption follows from creating these documents.
Task slicing follows the implementation concern map later; no work packages/lanes
are created during this plan phase.
