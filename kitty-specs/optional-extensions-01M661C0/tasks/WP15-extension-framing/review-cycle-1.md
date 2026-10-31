---
affected_files: []
cycle_number: 1
mission_slug: optional-extensions-01M661C0
reproduction_command:
reviewed_at: '2026-10-31T01:08:22Z'
reviewer_agent: codex
wp_id: WP15
---

# WP15 independent review — changes requested

Reviewer: codex / reviewer-renata. Reviewed delivery 96f13ab77869aad5ed35cc9c2a05abce40661e07 against assigned base c1ccc4172f8f95f6761a552181e1465ddfe77307. The scope is the approved host-v1 section 7.1 inert frame/message inspector. No session, handshake/order, live writer, signature, schema-semantic or business acceptance is claimed.

## R1 — Add the required exact mirrors for closed inventories (blocking, P2)

The review workflow's contract round-trip gate explicitly requires an assertion that mirrors each contract-pinned closed runtime set against its test/doc copy exactly. WP15 introduces closed sets without that assertion. Examples are the recovery inventory at `internal/extension/protocol.go:294`, the per-origin error inventories at lines 455–458, native admission values in `mutationResultRule`, and `unsupportedNames` at lines 581–582. The method inventory is reused through `methodNames`, but the new protocol fixtures also have no exact equality assertion against that runtime inventory. Other new typed enum inventories require the same check.

The current `internal/extension/protocol_test.go` constructs expected examples and submits them to `InspectMessage`; it checks accepted samples and selected invalid values. It does not compare the complete runtime sets to the frozen sets. The external `qualify.py` likewise has no exact inventory comparison. For example, appending an extra recovery member can leave every current fixture accepted and every current selected negative rejected. A correct inventory today therefore does not satisfy the explicit drift guard.

Required correction: use one authoritative private runtime value for each new closed inventory and add committed exact set/list mirror assertions against the frozen contract values (including no additions, omissions, duplicates or renamed members). At minimum cover method names, unsupported surfaces, origins and each origin's error codes, recoveries, native admission, and the other newly introduced closed typed enum sets. The runtime validators must consume those same values; do not create test-only copies of the runtime inventory, unused exported APIs or synthetic production callers. Retain production-boundary positive and precise negative controls. Demonstrate that a missing or extra member breaks the corresponding mirror, then rerun focused checks with exact final source/binary identities. No change to the frozen public vocabulary is requested.

## Independent evidence retained

Evidence directory: `/home/lynn/Documents/Codex/2026-10-24/wa/work/optional-extensions-WP15-review/`.

- Recomputed all 232 Go/module input digests and the input-map digest; they match the authored delivery. All 226 approved WP13 baseline inputs are unchanged. Exactly six owned additions are present. Handoff, supplied binaries and every listed evidence digest match. Authored-range diff check is clean; frozen source, baseline evidence and contracts are untouched.
- 188 independently written controls executed the supplied standard binary b78ee8668d1a51b5073aad01943908a98f7de3d69c7f2667327b273e31b96b2a and native-only binary 1a32b61cc7b0a44fa4928205b73375427a3fcf2bf964412a4af72460a52bcfaf. Positive file/stdin, exact public 8 MiB recipe/digest, first-over/zero/truncated/trailing/coalesced frames, invalid JSON/Unicode/keys/types/version, binding override refusal, request/response/cancel correlation, all frozen error codes/recoveries and native admission values passed with exact causes and no body echo.
- No-prefix, partial-prefix, partial-payload and final-EOF stalls passed with a 30 ms utility timeout for inherited blocking and nonblocking stdin in both variants. Maximum observed elapsed time was 35.4 ms. Parent stdin flags were restored. The focused race check of `TestExtensionFrameInputRestoresStdin` passed, as did actual supplied-binary `TestExtensionNativeReadDetector` for both variants. No repository refs/objects or private-state bytes changed.
- The original four stalled-stdin failures remain retained in author evidence; the reviewed correction owns/closes the pollable duplicate and restores the inherited blocking mode. No further behavioral defect was reproduced in these controls. Actual Mac/ARM execution and final platform/package/performance/full-suite qualification remain pending under their later owners.

## Contract walk

`host-v1.md` sections 2/4/6/7.1 are in scope: raw prefix/size/UTF-8, exact envelope and typed field/nullability shapes, frozen methods/errors/recovery/admission and the inspection-only correlation matrix were checked against code, public literals and live controls. `artifact-v1.md` contributes the scalar/identifier/schema/record/signed-object shapes and narrower finite bounds; signature verification, schema evaluation and business relationships remain explicitly outside this inspector. `conformance-fixtures.md` literal framing controls, exact padded-frame recipe/digest, and the typed public signed/native examples were walked against the relevant rules and supplied evidence. `default-workflow.md` contributes the no-loader/no-state-effect hard gate, exercised with the actual variant binaries. Session lifecycle/writer/capture/replication and final qualification examples are orthogonal to this slice. The exact inventory mirror gate fails as R1 above.

## Eight anti-pattern results

1. Dead code: PASS — `ReadFrame` and `InspectMessage` have current production calls from `extensionFrame`; the static dispatcher reaches that CLI and all new value/helper code participates in it. No unused public writer/session API.
2. Synthetic-fixture test: PASS — tests submit constructed or hand-encoded messages to production validation and the supplied executable; they assert its results/refusals.
3. Silent empty return: PASS — nil returns represent successful rule validation or typed absence; I/O and validation failures propagate sanitized structured errors.
4. FR coverage: PASS for the approved inert slice — request override rejection covers its FR-005 boundary; finite framing/timeout controls cover FR-019/NFR-003; typed structured refusals cover FR-020; static stdio/no-launch scope preserves C-009. This is not a claim to later session behavior.
5. Frozen surface: PASS — no authored modification to frozen existing source/contracts/evidence.
6. Locked decision: PASS — inert shape inspection, no registry/grant/module/store/native-state loading or effects, no unsupported execution/authority route. R1 is the separate mandatory contract-mirror gate failure.
7. Shared-file ownership: PASS — exactly six owned additions; no overlap with another WP's production paths.
8. Production fragility: PASS — no panic/bare raise/fallback success; explicit input and I/O refusals preserve error shape/privacy.

Verdict: changes requested, solely R1. WP16 depends on WP15 and must not consume this source as approved; rebase its lane after the corrected WP15 is approved. No implementation changes were made by the reviewer.
