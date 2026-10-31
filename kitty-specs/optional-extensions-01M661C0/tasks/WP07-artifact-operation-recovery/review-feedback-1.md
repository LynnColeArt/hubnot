# WP07 review 1 — changes requested

Actual independent reviewer: codex / reviewer-renata. Review prompt: `/tmp/spec-kitty-review-prompts/nichthub-2bfda21ece60/optional-extensions-01M661C0/WP07/86663544900c4f809baef7a2109a6564.md`.

Reviewed tested source `a3833afd1a4590ca502ef3916a2af1404003fec1`, metadata delivery `938f406d544475ec89a7c5f7632e63d535bd21f0`, and current lane inputs. All 252 Go/module inputs match map SHA-256 `7b96f8675727f4b1a625ed64d62b8a3e9e8d095c5a19145a25a2d8bb8b940997`. All 248 inherited inputs equal approved WP06 `ec99da3c0ad2fccd36a252a2313eeaa6b1d33b33`. This review does not approve the separately reopened WP05 explicit-source reader refactor or claim final combined-source qualification.

## R1 [P2] Preserve the frozen lookup result's replay flag

`contracts/conformance-fixtures.md:2025` pins `lookup_conditions`' post-publication/pre-ack `found` result to `result.replayed=false`. `artifact_operation.go:72` instead calls `artifactResult(history, evidence.Public, true)`, so a read-only operation lookup always emits `true`. `artifact_operation_test.go:74` asserts that conflicting value and therefore misses this contract drift. A status query reports the existing operation; an explicit identical mutation retry remains a separate action that can correctly return `replayed=true`. The contract allows current conflict/head-count observations to refresh at the returned cut, but does not authorize rewriting this literal lookup field.

Independent reproduction used both exact supplied executables, fresh real Git repositories, and an actual `record.open` process killed with SIGKILL after its real `update-ref` completed and before acknowledgment. The descriptor remained `publishing`; the subsequent read-only lookup returned `status=found` and `result.replayed=true`, contradicting the literal expected `false`. Exact snapshot comparisons confirmed the query itself had no effects. Explicit mutation retry returned the same original identity with `replayed=true`, as expected. Evidence is retained outside Git at `/home/lynn/Documents/Codex/2026-10-24/wa/work/optional-extensions-WP07/review-cycle-1/reproduction.json`, `reproduce.py`, and both `controls-attempt2-*/trace.json`. Frozen fixture SHA-256: `7b077bc80c62f4d76cd5cfeb71eacba7c1c3a26514d00c223e1a5b391fe6792e`.

Required correction: make the owned read-only operation projection match the frozen found-lookup result (`replayed=false`); retain `replayed=true` for actual identical mutation retry. Change the owned operation tests to assert those distinct behaviors through the live CLI, including post-CAS lost acknowledgment and public-only reconstruction. Keep the frozen contract bytes unchanged. Requalify both supplied variants against the corrected source and preserve the initial failing contract control and exact new source/binary binding.

## Verification and contract walk

- `artifact-v1.md`: reviewed operation selector, identity scope, strict eleven-field descriptor, settlement/absence/uncertainty, no-creation read behavior and live CLI. No additional defect found.
- `host-v1.md`: reviewed twelve-field OperationResult, ten-field MutationResult, nullability, type selector and closed status/reason vocabulary. Host grants/framing remain the later adapter's responsibility.
- `conformance-fixtures.md`: reviewed relevant publication/lookup literal states, recovery outcomes and input bounds. R1 fails this required round-trip check.
- `default-workflow.md`: this slice's ordinary native/default boundary remains unchanged; all inherited inputs match their approved snapshot. Final default/performance/platform/package qualification remains mandatory later.
- All 161 author evidence-index entries independently hash-verified. Both retained public traces independently verified four signed artifacts each (raw object/commit identities, Ed25519 signatures, actors, request digests, streams and sequence/previous links), 11 source cuts each, and 15 recorded no-effect calls each.
- Both supplied variants independently passed focused `go test -race -count=1 -json -run '^TestArtifactOperation' .` with `HN_WP05_BINARY` selecting the supplied executable: 41 pass events each, zero failures or skips. These green tests currently include the incorrect assertion identified in R1. No application rebuild or full suite was run by the reviewer.
- Fresh independent controls also passed verified absence without creation; changed-content/wrong-type refusal without effects; exact mutation retry identity; and genuine native identity rotation followed by `actor_mismatch` for both stale-pin replay and new write, while lookup still finds the original.
- Authored four-file BASE..DELIVERY diff check passed. The normal author gate's `no_coverage` result is unavailable Python coverage, not a Go pass or bypass.

## Eight anti-pattern checks

1. Dead code: PASS — static command registration reaches `cmdArtifactOperation`, which currently calls `inspectArtifactOperation`.
2. Synthetic-fixture test: PASS — FR evidence calls supplied production executables and real Git; the separate classification inventory unit test supplements those live paths.
3. Silent empty return: PASS — new query/command paths return explicit errors; no swallowed failure or empty success found.
4. FR coverage: PASS — live assertions exercise FR-007 recovery, FR-011 current conflict/heads, FR-018 public identities, FR-020 structured refusals/uncertainty and NFR-006 private-marker exclusions. R1 is a distinct frozen-output mismatch those tests must correct.
5. Frozen surface: PASS — only the four owned operation Go files were authored; inherited runtime and frozen contracts remain byte-identical. The documented generated-frontmatter cleanup and canonical lifecycle metadata are accounted separately.
6. Locked decision: PASS — no additional MUST NOT violation found in writer reuse, authority, native setup, signer/lock creation or private/public boundaries. Contract round-trip fails separately at R1.
7. Shared-file ownership: PASS — the four authored code paths are exclusively WP07-owned; approved dependency source is unchanged, and generated metadata is not attributed as WP07 implementation.
8. Production fragility: PASS — errors fail explicitly before unsafe projection/effects; no new panic/raise or transient-error-as-absence path found.

Verdict: REJECT, one actionable blocker R1. WP11 depends on WP07 and must consume the corrected approved operation slice and rebase/integrate its exact delivery; no downstream approval is implied. Heavy/application slot released before this report. No source or fixture fixes were made by the reviewer.
