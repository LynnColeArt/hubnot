# Mission Specification: Consumer CLI contract

**Mission Branch**: feat/consumer-cli-contract  
**Created**: 2026-11-04  
**Status**: Ready for planning review  
**Input**: Package Hubnot #40/#41 and work through them while preserving the default workflow.

## User Scenarios & Testing

### User Story 1 — Attribute exact candidate verdicts (P1)
A native consumer reads proposal show/status/log as versioned JSON, cites full identities, and joins Fleet-assigned reviewers itself.
1. Two actors share a display name: both verdicts remain separately keyed by full fingerprints and full review event IDs.
2. A newer verdict replaces the same actor's older sequence for exactly the candidate. A late old-candidate review never becomes successor evidence.
3. An absent approval exposes eligible reviewers and threshold deficit without inventing assignments or unanimity.
4. Superseded evidence can be ready while lineage prevents acceptance/merge; disagreement and eligibility remain separate facts.

### User Story 2 — Cite signed receipts (P1)
A consumer invokes review/decide with --json and receives the exact created event ID. The decision returns its stored full policy digest and evidence IDs. Evidence IDs match proposal status references and resolve through event reads; proposal status still takes a proposal ID.

### User Story 3 — Discover and resolve safely (P2)
A keyless user outside a repository discovers capability/schema versions without Git IO, locks, signer loading or bootstrap. A verified read resolves an event prefix or full ID; ambiguous prefixes return distinct native ambiguous_id. Artifact and mutation inputs retain existing full-ID rules.

## Requirements

### Functional Requirements
| ID | Title | User Story | Priority | Status |
|---|---|---|---|---|
| FR-001 | Versioned read envelope | Optional proposal show/status/log JSON exports full candidate, base/head, policy, lineage, evidence, requirements and pipeline observations through existing {ok,data,error}. | High | Open |
| FR-002 | Exact current reviews | Full actor-keyed latest sequence verdicts and full event IDs apply only to exact candidate; expose eligible actors and threshold deficits without Fleet assignments. | High | Open |
| FR-003 | Readiness semantics | Expose evidence readiness separately from disagreement, superseded/closed lineage, acceptance and merge eligibility; reuse native evaluation. | High | Open |
| FR-004 | Exact mutation receipts | review/decide --json return full created IDs; decide returns stored Evidence and Policy rather than recomputation. | High | Open |
| FR-005 | Event prefix read | Documented verified read/resolution uses existing resolveEvent ambiguity handling and native ambiguous_id; mutation and artifact prefix rules unchanged. | High | Open |
| FR-006 | Static discovery | Version/capability discovery advertises explicit schema/capability versions without repository, key, authority, locks, Git IO or bootstrap. | High | Open |
| FR-007 | Contribution consistency | Audit/document all nine contribution json flags and actual always-JSON outputs, preserving their default bytes; no incompatible envelope transition. | Medium | Open |
| FR-008 | Closed errors | JSON errors reuse docs/extensions.md vocabulary, consistent exit 0/1 and sanitized shape; operation null where no identity exists, no automatic retry/idempotency promise. | High | Open |

### Non-Functional Requirements
| ID | Title | Requirement | Category | Priority | Status |
|---|---|---|---|---|---|
| NFR-001 | Default compatibility | Absent --json stdout/stderr and exit behavior remain byte-identical for every affected existing command. | Compatibility | High | Open |
| NFR-002 | Public evidence | Public CLI tests cover every listed command in JSON/default modes, colliding names, exact candidate latest/late verdicts, missing thresholds, errors/unknown flags, keyless discovery and zero-effect refusals. Deterministic bounded ambiguity fixtures avoid brute force. | Verification | High | Open |
| NFR-003 | Bounded validation | Focused race + native fast/static before each review; affected integration/migration/protocol/trust/recovery before accept; full journeys stay integration, existing deadlines/assertions unchanged. | Validation | High | Open |

### Constraints
| ID | Constraint | Status |
|---|---|---|
| C-001 | No signed event bytes/schema, signature/history authority or policy semantics changes; pure read projection reuse. | Open |
| C-002 | Native core requires no plugin/service/provider; preserve verification/moderation/shallow/pending-history/refusal boundaries; reads append no signed facts and load/migrate no signer. | Open |
| C-003 | All mutation inputs remain full IDs; artifacts retain no-prefix rule. No operation-key idempotency or memory #21 expansion. | Open |
| C-004 | Source-only mission; leave releases/assets, #38 stress/soak qualification, Mac signing and original user checkout untouched. | Open |

## Success Criteria
All eight FRs have independently reviewed public contract evidence, existing native workflow checks pass, strict accept and local merge complete, fresh mission fidelity review and retrospective record precise source bindings and limitations. Root owns final PR/CI/main merge and issue closure.
