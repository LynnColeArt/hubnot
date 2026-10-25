# Implemented contracts

This index records the interfaces and acceptance boundaries already specified,
implemented and independently reviewed by this mission. It introduces no new
requirements. The source under review is
`4ee9ed8badcc29f4cd00aacd88534b9cf9cc2f23`.

## Native storage and authority

Issues retain their existing signed wire representation. Projects, project
items, discussions, discussion replies and workflows use explicit registered
native kinds in the existing signed actor histories and Git transport. Mutable
heads remain visible as conflicts; writes pin the expected revisions and actor,
and resolution is explicit. Project items pin an exact schema revision and
accepted discussion answers pin an exact reply revision. External authors,
times, identifiers and metadata are provenance claims, not native permissions.

The maintained contracts are [resources-v1](../../../docs/resources-v1.md),
[protocol-v0](../../../docs/protocol-v0.md),
[issues-v1](../../../docs/issues-v1.md),
[projects-v1](../../../docs/projects-v1.md),
[discussions-v1](../../../docs/discussions-v1.md) and
[workflows-v1](../../../docs/workflows-v1.md). Their typed states, limits,
revision guards and transport closure rules are enforced by the native model,
engine, command and replication tests.

## Public CLI

`hn gh issue` is a local parallel implementation of the documented issue
command/flag/JSON subset. It operates on the same native signed issues as
`hn issue`; unsupported semantics fail explicitly. Native project, discussion
and workflow authoring and reads are available without a forge. The compiled
native command registry is discoverable through `hn help`.

Native workflows preserve a versioned pipeline definition or an inert foreign
source document. Materialization is explicit and path guarded. A governed run
request checks the selected immutable workflow definition against the actual
proposal commit's canonical pipeline bytes and existing policy. Importing or
materializing a definition does not execute it.

Exact syntax and JSON boundaries are documented in the maintained guides above
and [github-interoperability](../../../docs/github-interoperability.md).
`TestInteroperabilityPublicAcceptance` exercises default and transport-excluded
binaries, native lifecycles, absent credentials/gh, keyless reads and
materialization, and real bare-remote reconstruction with immutable actor data.

## Explicit provider migration

`hn transfer import|export` selects the provider, project and resource
explicitly. GitHub is the first optional adapter; common coordination uses the
provider-neutral `internal/forge` interfaces and typed resource graphs. Source
identity is provider, endpoint and opaque record identity, independent of
mutable project selectors or display keys. Missing mappings and unsupported
semantics produce preflight findings or errors before their planned writes.

Transfers retain authenticated native snapshots and provenance baselines,
preserve local changes and reject competing edits. All future private journal
states must fit the declared budget before publication. Issue, discussion and
project export journals authenticate complete recovery state with the selected
native identity; a signature neither grants provider permission nor proves
remote success. Workflow publication uses an explicit destination branch/path
and blob compare-and-swap. An uncertain write requires exact positive remote
reconciliation; a negative scan does not authorize a blind retry. There is no
distributed exactly-once guarantee across independent publishers.

GitHub discussions require explicit supported category/history decisions;
historical answer mismatch is not silently rewritten. Projects map supported
typed fields, options, items and views, and reject incompatible populated
historical item schemas before publication. Unsupported schema, layout,
automation and permission semantics remain manual. Arbitrary Actions YAML is
preserved exactly as inert source; native execution and constrained provider
rendering require explicit mappings. Definition preservation does not promise
equivalent execution.

The interoperability/resource guides are the exact supported/manual matrices.
The provider client and adapter tests use local fake REST/GraphQL APIs. Current
complete GraphQL documents were independently checked against GitHub's public
schema; live provider mutation is not an acceptance prerequisite.

## Verification and independence

The integrated source passed `go test -race -count=1 ./...`, the complete
`nogithub` suite, default/tagged vet and builds, dependency exclusion,
formatting, diff checks and public acceptance. The transport-excluded binary
retains native operation and imported data without a GitHub account, token,
`gh`, service, database or Docker dependency. Native Git transport is available
independently of provider migration.

Every FR-001 through FR-020, NFR and constraint is traced to concrete source,
tests and documentation in the
[independent final integration review](../tasks/WP05-offline-acceptance-and-operator-documentation/independent-review-1.md).
That report retains original failing probes, corrections, passing verification
and the exact source/gate evidence. The canonical acceptance matrix records
the individual requirement verdicts; work-package status events record
independent approval. Public-main publication and PR merge are separate steps.
