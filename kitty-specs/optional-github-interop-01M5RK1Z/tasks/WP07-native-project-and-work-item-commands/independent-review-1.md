# Independent WP07 review

Verdict: APPROVED
Reviewer: codex-independent-interop-review, resolved reviewer-renata; no implementation edits
Reviewed source: bb926b2ac910fbe8cbbf75f6e772aef646db0903
Canonical review invocation: 5ab0e3cde3f74873881bb0ac51dd10e8
Scope: project_commands.go, project_commands_test.go and the authorized prior-registration restoration in resource_engine_test.go.

## Assessment
No actionable defect found. Native project and project-item handlers register through the approved core extension registry. Full-state input retains strict core validation; title/body convenience edits derive from the explicitly expected historical state, preserving fields, options, views and metadata rather than refreshing the current head. Item add pins the sole project head or an explicit full historical/competing revision. An operation retry reconstructs caller input with the original implicit schema pin; changed explicit input still conflicts. Full-state revisions can migrate item schemas with exact typed value validation. Archive retains all unrelated state.

The commands use core actor-pinned writes, actor-global operation keys, exact expectations and explicit all-head resolution. Concurrent state remains null with all revision heads. Typed issue, proposal and discussion targets plus draft and external targets are validated by the core, without provider imports or authority changes. Item positions use bounded canonical decimal strings and exact big.Rat comparison, with ID tie-breaking; conflicted items follow ordinary items by ID. Pagination binds the accepted-ref snapshot, selection and limit. Reads and help do not require a signing identity. CLI help describes full-state schema editing, implicit/historical pinning, replay and conflict limits. Mission-wide README/acceptance documentation remains assigned to WP05.

## Verification
- Independently ran all TestProject tests under race at the exact source: PASS (4.148s). These exercise signed repository mutations through run(), including fields/views, metadata preservation, all target types, malformed values, schema migration, competing heads, resolution, replay, high decimal precision and reads after removing the signing identity.
- Additional independent overlay probes under race: PASS (1.642s). A cursor cannot cross project selection; explicit original-schema operation replay succeeds after project revision; changing the explicit schema under that operation fails with unchanged refs; conflicted items list with null state and both heads after ordinary items instead of selecting one head's position.
- Independently inspected the exact commit and authorized test-only delta; diff check PASS. No production provider/network/process dependency found. Lane source remains bb926b2a; review created no production changes.
- Orchestrator supplied exact-source full repository race PASS (258.325s), vet, normal/nogithub builds, tagged tests (2.703s), dependency exclusion, formatting and diff checks. Default generic for_review gate completed no_coverage without bypass; unavailable generic Python/JUnit coverage is not a Go failure.

## Required anti-pattern checklist
1. Dead code: PASS. init registers cmdProject/cmdProjectItem through NativeResourceCommands; these invoke convenience, item mutation/list and output helpers, while generic paths call CmdResource. Targeted caller searches found live production paths for all new helpers.
2. Synthetic-fixture tests: PASS. Tests feed typed fixtures to real run(), core mutation/query and signed Git storage, then assert resulting state, operation replay, errors and unchanged refs. No literal response fixture substitutes for the implementation.
3. Silent empty return: PASS. Nil success returns occur in help/valid parsing; empty selections are ordinary pages. Errors from input, identity, state, graph and mutation boundaries propagate. Internal unmarshals in list operate only on already validated catalog states.
4. FR coverage: PASS. FR-014 has typed project/field/view/item, exact schema and target evidence; FR-016 has signed immutable replay/guards, conflicts/all-head resolution, source metadata preservation and native offline evidence, building on the approved core transport/admission tests.
5. Frozen surface: PASS. The exact source commit adds two owned command files and restores one test registration; no old signed bytes, protocol schema or frozen evidence changed.
6. Locked decision: PASS. No provider/service/gh dependency, authority import, coercion of invalid values, timestamp winner or implicit conflict resolution.
7. Shared-file ownership: PASS. The sole shared edit in resource_engine_test.go saves and restores the previous NativeResourceCommands project handler instead of deleting it. Root explicitly authorized this narrow integration cleanup from WP06; no core production helper changed.
8. Production fragility: PASS. Stale/conflicted expectations, invalid cursors/typed input and operation collisions return explicit errors. Historical input reconstruction supports retries after later edits; no panic or hidden execution fallback introduced.

This work-package approval does not declare the expanded mission complete or approve provider adapters in later WPs.
