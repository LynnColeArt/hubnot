# Acceptance evidence — Rich Issues and Reliable Automation

Candidate: `9f799ba5fb4e0b32067c3ff466ec29f97382544a` in the WP04 integration lane. Baseline: `a4d4bbfb9ba7275275b8d1bb281cf988baadeb9f`.

This records executed evidence and completed independent reviews. All four packages are approved; the canonical acceptance matrix and acceptance command remain authoritative for the final gate.

## Executed candidate gates

- `go test -race -count=1 -timeout=8m -v ./...`: exit 0, 246.221 seconds.
- `go vet ./...`: exit 0.
- `go build -o /home/lynn/Documents/Codex/2026-09-09/h/work/wp04-evidence/bin/ ./...`: exit 0; both main packages built outside the repository.
- Changed Go files formatted; staged and working `git diff --check` passed.
- Full race log SHA-256: `7fea1bfb7ad977417627195b2187dd0bd8e7d4df94d19b845efa22daf5bc1f59`.
- Full evidence and the ten-log digest manifest are retained in `/home/lynn/Documents/Codex/2026-09-09/h/work/wp04-evidence/`; a portable evidence archive will accompany closeout.

The short/full-ID documentation wording was corrected during the full run. Executable source and documented JSON did not change. Both corrected command sequences were separately executed in a disposable repository, recorded in `doc-id-guidance.json`.

## Measured reader behavior

The integrated 1000-issue fixture compared complete verified StoredEvent values and found exact equality. In the final race run, the old loader took 6.726181614 seconds with 3002 Git processes; the batched reader took 0.610863516 seconds with 5. Five public CLI pages covered all 1000 sorted issue IDs in 1.325664846 seconds with 25 Git processes, including five batches and five traversals.

This is an issue-only fixture measurement under the race detector, not a latency SLO. Mixed governance histories may require additional policy or pipeline validation calls. The accepted claim is bounded batched object ingestion with preserved verification semantics, plus actual public CLI routing through that reader.

## Requirement trace

| Requirement | Concrete evidence |
|---|---|
| FR-001 | TestOperationalRichIssues/HumanLifecycleAndReplay; TestOperationalRichIssueDocumentedInput; dependency TestRichIssueSignedCompatibility retains pre-rich signed bytes |
| FR-002 | TestOperationalRichIssues/HumanLifecycleAndReplay and Consumer: full rich state round-trip; TestOperationalRichIssueDocumentedInput consumes the actual docs JSON |
| FR-003 | HumanLifecycleAndReplay compares original payload/signature Git blobs; TestOperationalRichIssueReplication conflict cases preserve both parents and converge with four revisions |
| FR-004 | ConflictArrivalReverse=false/true expose state:null and both attributed heads; TestOperationalRichIssueStagedHeads inspects201heads and preserves every unconsumed head; dependency TestIssueCatalogPreservesAndResolvesConcurrentHeads covers timestamp/order invariance |
| FR-005 | HumanLifecycleAndReplay stale and changed-expect guards with exact zero-ref-write checks; conflict stale resolution; staged snapshot invalidation after comment; dependency TestIssueCommandsCapturedActorRaceAndOperationConflict proves deterministic actor CAS race |
| FR-006 | TestOperationalRichIssueReplication/DistributedGraphCycles tests local rejection and remote admitted blocks/parent cycles; conflict case validates incoming ambiguous edge/provenance; ExactSupplierAdmissionRecovery covers relation target closure |
| FR-007 | HumanLifecycleAndReplay tests create/revise/comment/close/reopen identical retries with zero refs changed, changed content/parent/target/intent key reuse errors; Consumer independently exercises retry; dependency TestIssueCommandsOperationScopeAndDuplicateHistory covers cross-actor key scope and duplicate-key ambiguity |
| FR-008 | Public subprocess helper requires schema hn.issue/1, one stdout JSON value, matching ok/error and process exit; StrictJSONAndRefInvariants covers raw invalidUTF8/unpaired surrogate, valid paired/U+FFFD escapes, malformed fields and bool aliases; Consumer imports no Hubnot internals |
| FR-009 | Public pages helper enforces complete total, stable snapshot and bounded pages; staged heads checks default50/max200 recovery; ThousandQuery proves1000unique sorted IDs; wrong query/limit/range, malformed and stale cursors reject |
| FR-010 | ExactSupplierAdmissionRecovery uses actual selected hn sync, tests missing root/parent/target without dependent promotion or selection expansion, then recovers exact fact; ThousandQuery compares complete StoredEvent structures between loaders and public inventory IDs |
| FR-011 | TestOperationalRichIssues/Consumer builds/runs examples/issue-consumer, validates returned IDs against repository, and tests invalid executable/repository; example has only standard-library imports |
| FR-012 | README plus docs/issues-v1.md/protocol-v0.md/threat-model.md and example README; TestOperationalRichIssueDocumentedInput executes the actual documented JSON/command journey; public conflict and staged cases execute documented recovery forms |
| NFR-001 | StrictJSONAndRefInvariants input/field bounds; staged/default page bounds; ThousandQuery max200 and default50; approved reader framing/count/byte and model aggregate-boundary tests run in full gate |
| NFR-002 | TestOperationalRichIssueThousandQuery logs integrated duration/process counts and exact old/new StoredEvent equality plus1000public sorted IDs over five bounded pages; TestIssueReadThousandIssueEqualityAndScaling separately compares1vs1000constantprocesses |
| NFR-003 | Full candidate go test -race -count=1 -timeout=8m -v ./..., go vet ./..., all-package go build to external directory, gofmt and staged git diff --check; full-race.log/vet.log/build.log |
| C-001 | No module/dependency/storage/service changes; consumer executes public CLI; all replication acceptance uses temporary ordinary bare Git remotes |
| C-002 | Issue state/assignment/criteria remain descriptive; no consumer execution, review or merge authority is added; documents and example explicitly preserve this boundary |
| C-003 | Only seven WP04-owned paths changed; integrated full suite includes proposal/CI/identity/replication/memory/namespace regressions; generated binaries/logs outside source, synthetic private keys only in disposable test repos |

## Public suite structure

- TestOperationalRichIssues/HumanLifecycleAndReplay
- TestOperationalRichIssues/StrictJSONAndRefInvariants
- TestOperationalRichIssues/Consumer
- TestOperationalRichIssueReplication/ConflictArrivalReverse=false
- TestOperationalRichIssueReplication/ConflictArrivalReverse=true
- TestOperationalRichIssueReplication/DistributedGraphCycles
- TestOperationalRichIssueReplication/ExactSupplierAdmissionRecovery
- TestOperationalRichIssueStagedHeads
- TestOperationalRichIssueThousandQuery
- TestOperationalRichIssueDocumentedInput

The controlled201head fixture uses synthetic valid signed actor histories solely for efficient setup. All head inspection, pagination, stale guards, partial resolution and final convergence under test use the compiled public CLI. Normal lifecycle/replica/supplier mutations and sync use the public CLI throughout.

## Contract refinements verified during acceptance

Wrong-query, changed-limit, malformed and out-of-range cursors are invalid_input; changed accepted-ref snapshots are stale_cursor. Partial-resolution stale snapshot is stale_revision. This matches the independently approved implementation; no dependency source changes were required.

Requests generated from Go state structs must use empty arrays/maps or omit optional fields; explicit null is rejected. Acceptance helper serialization now emits valid empty collections. Missing supplier diagnostics name an exact unavailable fact but do not prioritize root before other missing references; tests isolate each supplier case in a fresh receiver.

## Independent review disposition

WP01 approved source `cb52d48ee80d57868f5311b66d32f8c8457fd2f1` after correcting the raw signed-payload cap; generated executable cleanup is retained at `2f79abb0`. WP02 approved `adcc6e7ad5486db271ec04e270e0aa2534fb5ceb` after correcting subprocess-cause preservation. WP03 approved `43d5c0f795b297cf3b56bbd3ca1daaee176e61ad` after correcting machine-flag grammar and malformed Unicode input. The independent original reproductions passed after each fix.

WP04 independent reviewer `codex-independent-acceptance-review` approved `9f799ba5fb4e0b32067c3ff466ec29f97382544a`. Representative public acceptance with race detection passed in 10.811 seconds; the reviewer assessed the 201-head and 1000-item assertions, complete gate evidence, documentation and all eighteen requirement mappings as adequate. All eight required review checklist items passed.

Canonical approval events: WP01 `01M22HFRBNTENVJFA6YMFDCXA7`; WP02 `01M22H9808621AD3KE0JEXWEE6`; WP03 `01M22KW00ZEWVYA4QDYBX6GPRH`; WP04 `01M22N92YCPKHMJPJTDQ7GWVDE`.

The host recorded these actual independent verdicts through the canonical transition command. Its force flag addressed only the recurring inherited planning-metadata hygiene false positive. Independent review, source ownership, actual Go quality gates and acceptance were not waived. The Python architectural auto-gate reported unavailable coverage in this Go repository; this is not claimed as a passing Python gate. See traces/tooling-friction.md and each review cycle.
