# Consumer contract acceptance trace (all five WPs independently approved)

Authoritative requirement prose: kitty-specs/consumer-cli-contract-01M6JP9A/spec.md. All eight FR verdicts were recorded through canonical acceptance-verdict after final independent approval; overall verdict pass. Strict local acceptance6610a8c7 passed. This table maps the full15 requirements to evidence; it does not replace canonical state.

| Requirement | Evidence selectors |
|---|---|
| FR-001 full proposal/log projection | TestConsumerProposalProjectionCLI; TestConsumerEventAndLogCLI; TestConsumerPublicJourney |
| FR-002 exact attributed latest reviews | TestConsumerProposalProjectionCLI; real colliding-name actors, untrusted observation, late predecessor review |
| FR-003 distinct evidence readiness/lineage/observed preconditions | TestConsumerProposalProjectionCLI; protected checks, pipeline results, missing thresholds; observed preconditions never authorize actor/target |
| FR-004 exact stored receipts | TestConsumerReceiptsCLI; exact stored Policy/Evidence after later history; status with proposal ID and event read with evidence ID |
| FR-005 verified prefix read and ambiguity | TestConsumerEventAndLogCLI; TestConsumerEventResolverAmbiguity; TestConsumerEventModerationCLI |
| FR-006 static keyless discovery | TestConsumerStaticDiscoveryCLI; TestConsumerOutputGrammarCLI |
| FR-007 contribution output consistency | docs/consumer-cli-v1.md nine parse-site audit, aliases and existing default JSON; WP01 independent review; contribution implementation unchanged |
| FR-008 closed errors and safe recovery | TestConsumerProposalErrorsCLI; TestConsumerReceiptEarlyRefusalsCLI; TestConsumerOutputGrammarCLI; native ambiguous_id; operation:null/do_not_retry |
| NFR-001 default bytes | Static, proposal, log, review/decision exact output assertions and original parser/help inventory; default/native journey |
| NFR-002 public behavioral coverage | All11 consumer roots in bounded integration, actual default/native children, visible successor/hidden predecessor control, deterministic bounded ambiguity fixture |
| NFR-003 bounded validation | Focused race + native fast/vet per WP; final integration/migration + protocol/trust/recovery; old fast21/integration8 retained, no deadlines/assertions weakened |
| C-001 immutable signed authority | Event schema untouched; stored receipts only; AuditOldEventEncodingPreserved + native policy/lineage regressions |
| C-002 native trust/read boundaries | Moderation/admission read selection; keyless/no migration reads; shallow/identity and native pipeline regression evidence |
| C-003 full mutation/artifact IDs, no idempotency expansion | TrustBearingCommandsRejectUnambiguousPrefixes; no artifact edits; unkeyed errors operation:null; existing late writer migration retained |
| C-004 source-only scope | No release/assets/original-checkout edits; exhaustive stress/soak #38 and memory recovery #21 deferred to actual followups |
