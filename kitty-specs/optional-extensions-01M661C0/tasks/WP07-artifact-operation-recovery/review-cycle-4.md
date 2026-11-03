---
affected_files: []
cycle_number: 4
mission_slug: optional-extensions-01M661C0
reproduction_command:
reviewed_at: '2026-10-31T18:33:53Z'
reviewer_agent: codex
wp_id: WP07
---

# WP07 cycle 3 independent review — REJECTED

Reviewer: codex / resolved reviewer-renata. The canonical reviewer profile and action-scoped charter were loaded through spec-kitty. This is an unconditional request for changes on the lazy-validator maintenance, not a reversal of the historical cycle 2 implementation evidence.

## R1 [P1] Keep every current validator caller safe before operation inspection

**Affected change:** artifact_operation.go:28-38, commit 08ca97325f6046f1900de8cbe68301e4e6f89b9c (delivery aa23d1074283a9c1e8a26a1aa1697579ebc1dfdb; review lane HEAD 3404a5e6b4f5e18d47465fffc86f270a15ecfe60).

The change makes artifactOperationType a nil *regexp.Regexp and initializes it only inside inspectArtifactOperation. The already integrated production callers cmdArtifactList (artifact_query_commands.go:103) and cmdArtifactSearch (artifact_search.go:74) directly call artifactOperationType.MatchString before that inspector. Every fresh CLI invocation with --type therefore dereferences nil. Operation lookup in a separate CLI process cannot initialize these processes. Unsynchronized direct reads would also be unsafe against concurrent first initialization if these paths are invoked together; only the deterministic nil crashes are claimed as executed evidence.

**Independent causal reproduction:** archived exact integrated commit 2d92ff4525fbafccf79ea83391e1dcfc147b19a5, built CGO_ENABLED=0 with -p=1 -buildvcs=false, first unchanged and then using a Go overlay replacing only artifact_operation.go with the exact candidate bytes (SHA-256 267e1b5ef70210dfad96d5e2a9dffacdbf9c41a005938b7b8ce5a1bf639033e4). No authored source was modified. In a real newly initialized Git/Hubnot repository, run each command as a fresh process:

- hn artifact list --type example.test/note --limit 1 --json
- hn artifact search --type example.test/note --literal needle --limit 1 --json

The unchanged integrated binary exits 0 and emits the verified empty Page with the same source cut. The overlaid candidate exits 2 with `panic: runtime error: invalid memory address or nil pointer dereference`, pointing respectively to artifact_query_commands.go:103 and artifact_search.go:74, and emits no JSON. Repeating both with reserved type hn/note gives the unchanged binary's exact invalid_input/do_not_retry refusal (exit 1), while the candidate panics again. All repository file hashes were unchanged across all eight calls.

**Impact and contract:** supported public list/search commands crash for any explicit type, including otherwise meaningful reserved-type refusals. This violates FR-020 structured diagnostics and the existing supported caller contract. The inspector-only race suite cannot reveal these later integrated direct readers. Treat this as a regression introduced by WP07, not work deferred to another owner.

**Required correction:** retain lazy initialization without exposing a nil or unsynchronized validator to any existing .MatchString caller. Preserve the exact regex literal and reserved namespaces, actor/operation/request/replay semantics, error codes/messages, bounds and native-startup constraint. Resolve this inside owned scope with a compatible safe lazy interface, or coordinate any indispensable scope change explicitly before editing another owner's file. Add fresh-process public list/search valid and invalid-type checks against the integrated caller set, and meaningful first-use concurrency coverage if callers share access. Re-run the causal startup RED/fixed control and the affected public operation/list/search paths with the exact standard and combined native-only binaries. Do not weaken refusals, eagerly compile at native startup, change frozen contracts, or waive performance budgets.

## Eight mandatory anti-pattern checks

1. **Dead code — PASS.** No new public function/API was introduced; inspectArtifactOperation retains its live cmdArtifactOperation caller (artifact_operation_commands.go:49). Integrated extension_artifact_adapter.go:623 also calls the inspector. Targeted final-union grep exposed the two additional direct validator readers above.
2. **Synthetic-fixture test — PASS.** The new startup test executes the production inspector and checks exact artifact.Error codes/messages across 32 concurrent callers. Independent reproduction runs actual built hn processes and real Git state; removing the candidate overlay removes the observed crashes. It is not a copied-result fixture.
3. **Silent empty return — PASS.** The production diff adds only sync.Once initialization and preserves existing error/result returns; no new swallowed error, empty result or unsupported-success path appears. The regression is a panic, not a fabricated empty success.
4. **FR coverage — FAIL.** FR-007 and FR-011 historical lookup/replay/restart coverage is retained in TestArtifactOperationPublishedLookupAndRetryCLI, FoundAndSourceCutCLI and InterruptedRestartCLI; FR-018/NFR-006 retain public projection, actor-pin and leak assertions. FR-020 is now observably broken: actual list/search valid input and reserved-type refusals panic instead of the frozen structured outcomes. The added inspector-only test does not exercise these production readers.
5. **Frozen surface — PASS.** The maintenance code commit changes only artifact_operation.go and artifact_operation_test.go. No signed fixture, protocol contract, Go dependency, native writer, frozen golden or public byte format changes. All 252 delivered Go/module input hashes match the retained tested-source map; the exact literal is unchanged. The integration reproducer uses the existing 2d92 base with only the owned source overlay.
6. **Locked decision — FAIL.** Exact regexp and startup laziness are retained, but the new nil state contradicts preserved supported command/refusal behavior and FR-020 diagnostics. Native dependency, grant, signer/journal and immutable-request decisions were not otherwise changed. No budget relaxation or full RSS pass is inferred.
7. **Shared-file ownership — PASS.** The maintenance production/test paths are WP07 owned; the four planning-only changes in aa23d107 are explicitly root-owned guard reconciliation in the handoff note and retained planning-guard-reconciliation evidence. No reviewer source edits or other WP status changes were made. Existing later callers remain integration obligations of this introduced shared-validator change.
8. **Production fragility — FAIL.** Although no new explicit panic/raise statement was added, nil *regexp.Regexp causes deterministic process panics in both supported CLI handlers before repository work or structured refusal. The unguarded direct access also leaves a potential initialization race. The fresh control/candidate comparison proves the actual crash mechanism.

## Evidence, scope and status

External evidence: /home/lynn/Documents/Codex/2026-10-24/wa/work/optional-extensions-WP07-lazy-validator-review/ (source-binding.json, union-base-inputs.json, owned-overlay.json, builds.json, reproduce.py, reproduction.json and per-command stdout/stderr).

Original supplied standard binary SHA-256 141d70cc0852287880394aab4102cb2f9d5b6bf8457fc705bdf031649a74cf9d; combined native-only 5596b2b1a1d8057b5830c411ed5ccf3d28ace4797bbf44c3ab40b141ebb5fcc7. Independently recomputed source map SHA-256 2b635636e72a9b92574195fba1cd522ec3ab35cb6e069377818aff657760745c, all 252 inputs unchanged. Causal integration control binary SHA-256 8a597eb039d6004c830c3bfb7279b8f29f9a88c3048a3723b9cc3d7ba69d9964; lazy candidate binary ecbcd549f902ccc38dfa62fa28db3525add98fbc61e29fb21142511cf4f7d8c1.

WP01 approval 01M66767D87RQVAEF5NMB31BQK, WP06 approval 01M67Y98AH1W8XXVX0P30SY629 and historical WP07 approval 01M680GG9XKPJHT0K7QYQ8TP1C were read from the coordination event authority. Fresh WP07 review claim is 01M69KH97PHQGR8ZJ611JMKN9F. Existing Python architecture gate outcome no_coverage remains unverified, not a Go failure or pass. Prior inspector-focused race evidence was inspected and bound but not rerun after the decisive public regression. All reviewer build/hn/Git subprocesses were reaped and the sole heavy slot released to root. No full RSS, package matrix, platform/adopter, mission, beta, main or release acceptance is claimed. Final integrated RSS qualification remains WP24 work.
