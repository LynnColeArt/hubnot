# WP14 review 1 — changes requested

Reviewer: codex / reviewer-renata. Reviewed delivery `4b1a98d6a558079b3c7f8c39c2c207e09c0e32ae` against implementation base `f48ecf1ee91add6bb9217c5981c9951a1baad0fe`.

## Finding 1 — P2: enforce the closed artifact.read selector/action pairing

`internal/extension/grants.go:146-147` (`methodAllows`) accepts `raw.inspect` for every custom selector. Consequently, `Authorize` accepts `Access{Method:"artifact.read", Selector:"artifact:example/type", Action:"raw.inspect"}` with only `artifact:example/type=raw.inspect` granted. This contradicts `contracts/host-v1.md:289`: ordinary custom reads require `read` on the matching custom type; opaque reads require `raw.inspect` on `artifact:opaque`. Lines 312-317 further distinguish those request forms. The exported normalized boundary must reject the unsupported pairing instead of relying on later adapters to avoid constructing it.

This is observable through current production callers. With a valid manifest declaring only `artifact.read`, both exact supplied standard and combined-native binaries successfully execute:

```text
hn extension enable example.inventory/tool --grant artifact:example/type=raw.inspect --json
hn extension inspect example.inventory/tool --json
```

Inspection reports `enabled:true` and that grant. `ValidateRegistrationGrants` treats the invalid pairing as a matching declared method. No module was started, and this review does not claim an already implemented host data-disclosure path.

An independent external Go overlay test reproduced the authorization defect without modifying repository source. The same test first passed concrete-type/read and artifact:opaque/raw.inspect positive controls, then confirmed concrete-type/read is denied with only raw.inspect granted, before failing because concrete-type/raw.inspect was accepted. Retained evidence is external under `optional-extensions-WP14/review-renata/` (`grant_matrix_test.go`, `overlay.json`, `grant-matrix.log`, and `raw-inspect-cli.json`).

Required correction: constrain `artifact.read` to the exact contract pairs and add regression assertions at `Authorize`, registration-grant validation, and the live enable/inspect boundary. Preserve valid concrete-type read and explicit artifact:opaque raw inspection. An otherwise supported grant value must not be presented as applicable to a method that cannot consume it.

## Finding 2 — P2: add the required closed-set mirror assertions

The new closed sets `nativeFamilies` and `grantActions` at `internal/extension/grants.go:48-49` have no test that compares their complete contents with the frozen contract. `grants_test.go` samples valid and invalid values, but does not detect every added or removed family/action. For example, adding an unrelated action to `grantActions` can leave all current assertions green because the eighth-action test independently fails the length limit.

The required review skill's contract round-trip check explicitly requires an assertion mirroring a closed runtime set against its test-side or document-side copy. The relevant contract sets are in `contracts/host-v1.md:58-63` and `:220-221`.

Required correction: assert the exact six native data families and exact seven grant actions, with no missing or additional members. Add a contract-derived selector/method/action matrix that rejects unsupported cross-pairings, including Finding 1, rather than only testing a few examples.

## Verified evidence and scope

- Independently recomputed every supplied evidence digest and both binary digests. Standard: `76e11ab0dad037388109362f99f1807e54fd6b024f291cbbf6b07b0a17bb113c`; combined-native: `d6e346d8130c36cb4d0aeaaa476b7ff746858855e1aa28680af3ca68e6541884`.
- All 230 current Go/module inputs match the supplied map `c596f96428bb9421efb5977d8e349794dac96e330d9ea86c7cf42eb6890f243b`. All 226 base inputs are unchanged; exactly the four owned files were added.
- Independently reran `TestExtensionEnablementCLI` with `-race -count=1` against each supplied binary, using combined tags for the native variant. Both passed. Existing passing tests therefore do not catch Finding 1.
- Authored-range `git diff --check` passes. Author's retained initial meaningful RED, focused internal race, vet, native registry-read/start controls, strict-storage bounds and exact binary evidence were inspected. No full suite or rebuild was repeated.
- `host-v1.md` registration/repository/grants and management examples were reviewed. Its framing/session, request adapters and live cleanup belong to later WPs. `default-workflow.md` applies to the inert dispatch boundary; final performance/platform qualification remains later work. Artifact signed-byte fixtures are orthogonal to these four files; opaque selector and grant-bound examples remain relevant and were checked.
- WP16 owns live cleanup, idle/request rechecks and wrapper integration. Their absence is not a finding against this slice. The changes requested here concern the current grant boundary and its actual management callers.

## Eight anti-pattern results

1. **Dead code — PASS.** Production enable/inspect/list callers reach grant validation and `Authorize`; disable/remove use the store. No unused session API was added.
2. **Synthetic-fixture test — PASS.** Tests exercise production authorization/storage code and supplied executable CLI paths.
3. **Silent empty return — PASS.** Success and missing-state returns are explicit; malformed private state refuses rather than being repaired.
4. **FR coverage — FAIL.** FR-005's exact scope mapping is incomplete: Finding 1 passes an unsupported selector/action pair, and current tests miss it.
5. **Frozen surface — PASS.** All 226 base inputs remain byte-identical; no historical/native file was authored by WP14.
6. **Locked decision — FAIL.** Finding 1 contradicts the frozen host method/grant matrix; Finding 2 leaves required closed-set conformance unpinned.
7. **Shared-file ownership — PASS.** The delivery changes only the four WP14-owned paths; no overlapping implementation edit.
8. **Production fragility — PASS.** No new panic/raise path; invalid state and busy storage return explicit structured errors before mutation.

WP16 depends on WP14. It must consume the corrected approved WP14 source after rework and rebase/reintegrate if it has already taken the rejected delivery. Reviewer made no implementation changes. Verdict: reject to planned with this feedback.
