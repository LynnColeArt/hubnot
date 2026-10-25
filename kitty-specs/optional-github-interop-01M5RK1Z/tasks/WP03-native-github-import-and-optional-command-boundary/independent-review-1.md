# Independent WP03 review

Verdict: APPROVED after pre-handoff corrections
Reviewer: codex-independent-interop-review (resolved reviewer-renata; separate from implementation)
Initial reviewed source: 4ccfafe7c510105fa195ddb0fb9a81319dfd7adc
Final reviewed source: bf0ca8304a090a11ea57238b7cb57e94fba12676
Canonical review invocation: 0114883a0ad24146b96e495be516e296
Scope: provider-neutral import/model/command/checkpoint files, optional GitHub adapter/stub, main routing and the explicitly authorized one-line gh_compat.go metadata-key alignment.

## Findings and dispositions
R1 (P2), accepted/fixed: the initial implementation trusted a syntactically valid but editable forge/baseline. Independent native-repository reproduction changed local managed content and manufactured its baseline, then observed an upstream competing edit overwrite the local work. The final code traces the signed resource ancestry to exact producer-shaped import operations and requires current baseline plus observation to match a unique nearest witness. The original independent test now passes, preserving native refs and local work on tampering.

R2 (P2), accepted/fixed: initial substring detection treated ordinary issue prose containing `\n\n<!-- hn:forge-issue:` as actual export provenance, even without any footer terminator/schema. Independent reproduction rejected valid source text. Final code recognizes only complete terminal provenance with strict schema, native IDs/actor and exact content digest; literal marker-looking prose imports unchanged, while actual unproven reverse export mapping still refuses. Original independent test now passes.

## Acceptance assessment
No remaining actionable findings. Neutral forge interfaces retain opaque provider IDs separately from selectors; generic coordination does not import GitHub DTOs. GitHub factory/credential selection is optional and !nogithub; native and local compatibility commands work without provider access. Import collects complete issue/comments data and validates all native payloads before that issue's first mutation. Existing immutable opening provenance remains authoritative for source mapping; repeated imports retain native-only criteria/relations/metadata and guard competing changes. Comment provenance requires exact structured header/footer/body digest and is checked across signed history, preventing duplicate copies after actor change or checkpoint removal. Native operation recovery retains exact signer/input/expected parent and refuses different pending intent. Selection-bound scope prevents key 8 from replaying pending key 7. Private checkpoint files use bounded strict JSON, private modes, atomic replacement and explicit single-local-transfer locking. Dry-run writes no native facts/checkpoints. Capabilities accurately report staged unimplemented mappings before factory/network/write.

The new localCommands fallback registers compiled local families only and cannot override existing switch commands. Repeated --map has strict nonempty/unique/length/count limits; issue transfer refuses unsupported maps before constructing a provider.

## Verification
- Initial independent existing-import focused run: `go test -count=1 -run '^(TestGitHubImport|TestForge)' .` PASS (1.946s).
- Initial independent R1 overlay: FAIL as expected, local title overwritten (0.220s).
- Initial independent R2 overlay: FAIL as expected, literal source prose refused (0.031s).
- Final-head independent Go-overlay probes for R1/R2, 64/65 mapping bound, issue-map pre-factory refusal and inability of local extension to override version: PASS (0.280s).
- Implementer final source gates accepted by orchestrator: full race PASS (230.752s), focused race PASS (5.514s), vet/default build/nogithub build/dependency exclusion/format/diff PASS. Default pre-review generic gate completed without force/skip; its no-JUnit/Python coverage baseline is unavailable rather than a Go regression.
- Final source head rechecked against bf0ca830 and all reviewed files: no difference. Canonical for_review event was 01M5RPBC75N83N60WY7QSXVTZE.

## Required anti-pattern checklist
1. Dead code: PASS for this delivery. main invokes cmdGH/cmdTransfer/cmdGitHub; transfer imports instantiate/use neutral provider and real GitHub adapter. Export hook and native local-family registry are explicit staged extension contracts owned by subsequent WPs, not claimed complete features.
2. Synthetic-fixture tests: PASS. Tests invoke public command routing, actual native signed storage and HTTP provider requests. Source models are fixture inputs, not substitutes for production responses.
3. Silent empty return: PASS. Empty source/native collections mean complete bounded emptiness; malformed provenance, duplicate claims, stale cursor, missing intent or invalid checkpoints produce explicit failures.
4. FR coverage: PASS for FR-003/004/005/008/009/011/012 and NFR-001 at WP03 scope; future native resources/adapters are separate accepted work packages.
5. Frozen surface: PASS. No signed wire/history mutations. Core issue behavior unchanged; gh_compat.go only changes the explicitly authorized source URL metadata key.
6. Locked decisions: PASS. No mandatory provider, gh process, service/database, implicit background sync or credential storage. External author/time remain attributed source claims.
7. Shared-file ownership: PASS. Primary WP ownership now names internal/forge/provider.go/test. One-line gh_compat.go key alignment is explicitly approved in WP prompt. main routing owns local fallback so later native WPs register without editing main.go.
8. Production fragility: PASS. Conflict, invalid source, interrupted journal, unavailable provider and resource-bound errors fail explicitly; no silent winner/retry or swallowed provider failure found.

## Integration obligations
Approval covers issue import and extension seams. WP04 supplies actual issue export; WP06 onward supply native project/discussion/workflow resources and their provider adapters. Current capabilities intentionally refuse those pending mappings. Final docs and full merged public acceptance remain WP05 obligations. Multi-event imports preserve valid partial progress on interruption and make no atomic/distributed-lock promise.
