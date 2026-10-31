# Keep optional registrations behind explicit use

2026-10-31 — root, architect-alphonso. Prospective bounded integration authorization; no new implementation or performance pass is asserted.

The frozen414d gate retains all4,320 observations and fails two native-only Go-only issue-show RSS groups by136/132KiB. Earlier2d92 and76cd runs also remain failures:12,960 observations total. Budgets remain max(10%,5ms) elapsed and max(10%,1MiB) RSS. All old results remain immutable.

Bounded profiling and independent architecture preflight identify18 eager optional registration initializers. Consolidate their FINAL effective bindings into one zero-state private Once bootstrap. The only eager action is binding the two existing native router keys. Construct the four existing maps only at actual explicit entry points; no repository/config/provider/module/signer/state access in the bootstrap. All original command names, argument captures, help/inspect/cleanup overrides, errors, authority/order, signatures and data remain exact.

This is a lifecycle bug fix with unchanged surface names, not a terminology/path/API rename; the bulk-edit rename workflow is inapplicable under its explicit surface-preserving bug-fix exception. Every initializer occurrence was nevertheless classified by Go AST; ordinary native/forge/migration/resource init functions are excluded.

## New ownership

`optional_bootstrap.go` and `optional_bootstrap_test.go` are new WP24 owned/create-intent paths. Existing authoritative surface remains extension_default_acceptance_test.go. Do not append old paths to owned_files (that would collapse active allocation lanes); do not remove original ownership or authoritative surfaces.

## Exact coordinated old-path exceptions

| Path | Original owner | Allowed initializer region |
|---|---|---|
| artifact_closure_commands.go | WP08 | original lines10–10: remove only the optional registration init body |
| artifact_commands.go | WP03 | original lines14–14: remove only the optional registration init body |
| artifact_envelope_commands.go | WP03 | original lines11–11: remove only the optional registration init body |
| artifact_exchange_commands.go | WP10 | original lines14–18: remove only the optional registration init body |
| artifact_operation_commands.go | WP07 | original lines9–9: remove only the optional registration init body |
| artifact_query_commands.go | WP11 | original lines18–22: remove only the optional registration init body |
| artifact_replication.go | WP09 | original lines17–20: remove only the optional registration init body |
| artifact_revision_commands.go | WP06 | original lines10–17: remove only the optional registration init body |
| artifact_schema_commands.go | WP04 | original lines15–19: remove only the optional registration init body |
| artifact_search.go | WP12 | original lines18–22: remove only the optional registration init body |
| artifact_store_commands.go | WP05 | original lines14–18: remove only the optional registration init body |
| extension_artifact_adapter.go | WP18 | original lines19–23: remove only the optional registration init body |
| extension_changes.go | WP19 | original lines39–43: remove only the optional registration init body |
| extension_commands.go | WP13 | original lines23–23: remove only the optional registration init body |
| extension_enablement.go | WP14 | original lines12–20: remove only the optional registration init body |
| extension_host_command.go | WP16 | original lines29–35: remove only the optional registration init body |
| extension_native_adapter.go | WP17 | original lines19–23: remove only the optional registration init body |
| extension_protocol_commands.go | WP15 | original lines15–15: remove only the optional registration init body |
| extension_changes_test.go | WP19 | TestExtensionChangesRegisteredMethodSet: prepend the bootstrap ensure before inspecting installed keys; retain all assertions |

Four declarations only: artifactCommands/artifactTypeCommands/extensionSubcommands/extensionHostMethods become typed-nil zero state in their existing files. Four existing map readers only gain a first-statement ensure: cmdArtifact, cmdArtifactType, cmdExtension, extensionHostDispatch. No other business body changes. Directly caused seam-comment/import cleanup is permitted.

The sole bootstrap Once must finish all table writes before any reader proceeds; it calls no entrypoint, external handler, process, filesystem, registry, provider or signer. No reset during concurrent tests. Fresh child processes prove cold first-use. All existing internal capability/session code remains unchanged: capabilities.list is host-owned, not an adapter entry.

## Frozen binding evidence

The committed companion binding and region JSON files are exact copies of the independently source-derived414d inventory, matched to one actual original source-assisted runtime snapshot.21artifact commands,3type commands,8extension subcommands,13host adapters and2native router keys are required. Closure captures need actual behavior tests; function PCs alone are insufficient. The final artifact inspect is artifactExchangeInspectCommand; help/--help is cmdArtifactSearchHelp; extension inspect/list are extensionInspectEnablement; enable/disable/remove are cleanup wrappers.

## Execution and review boundaries

All affected original owners are approved/inactive at authorization (events captured externally); WP24 alone edits these exact regions in its existing mapped lane. Any concurrent owner reopening pauses edits on that path. This is the existing root-coordinated out-of-map exception, not a formal whole-file transfer. Normal installed warn-policy diagnostics stay visible; no policy/config change, force, skip, no-verify, hand status/event edit or new checkout mode. A real blocking guard requires a supported documented resolution before proceeding.

The author commits one fixed candidate with a complete before/after AST scope/body/binding proof and focused unit/race/live CLI/host checks. A fresh independent source-only correction review precedes root source attestation and new measurements; that review does not approve WP24 or waive missing performance. All3old captures remain failures. After new source freeze use the complete paired gate first, then every original required functional/race gate. Normal final WP24 approval remains independent and mandatory.

No source-layout padding, global GC/runtime tuning, compiler flag switch, omitted feature, sidecar, extra native dependency, API rename or budget relaxation. Preserve default native workflow and source/data trust boundaries.
