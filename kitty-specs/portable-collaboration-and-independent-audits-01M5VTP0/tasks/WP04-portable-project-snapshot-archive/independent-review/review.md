# Independent WP04 review

Reviewed source: `41dc551c4d84624334b4d48bed994410129601bf`, lane-c. Reviewer: independent Codex review agent `/root/project_snapshot_review`. Result: approve; no actionable blocker found.

The complete runtime prompt, AGENTS.md, charter, spec, plan, research, data model, quickstart, all three contracts, WP04, lane ownership, six changed files and production admission helpers were inspected. The reviewed diff changes only owned WP04 files and preserves existing signed/native protocol implementations, release surfaces, dependency files and defaults.

## Verification

- Reused the completed source-bound full `go test -race ./...`: race.head equals the reviewed source; race.exit is 0; final root package pass is 473.867s. The exact log is retained here. No duplicate full suite was launched.
- Independently ran `go test -race ./internal/migration -count=1 -v`: pass, 1.047s. Covers deterministic attributed archive roundtrip, exact source bytes/numeric precision, required closed issue/comment schema fields, unsupported credential URLs, parent/blob/native alias dependency failures, positive budgets, hostile archive members and corrupt digests before any archive output.
- Independently ran `go test -race . -run '^TestProjectSnapshot' -count=1 -v`: pass, 2.334s. Covers selected exact signed payload/signature, unmerged proposal/history/policy closure, private/unselected/working-tree exclusion, missing fact/proposal suppliers, native corruption/budgets, unchanged source refs, keyless accepted observations and no quarantine refs/private identity directories.
- Additional independent tests use a Go overlay, leaving product code unchanged: `go test -race -overlay <this-directory>/overlay.json . -run '^TestIndependentWP04' -count=1 -v`: pass, 1.232s. Real SHA-1 and SHA-256 temporary repositories retain two selected commits; removing a real ancestor or introducing an unrelated object with a valid hash both refuse. Each hostile case asserts source branch stability. Reproduction source and full log are retained.
- Independent `go vet ./...`, `go build ./...`, formatting and `git diff --check`: exit 0, empty diagnostics. Orchestrator also supplied successful ordinary/nogithub/noforge focused checks; this review does not reinterpret noforge as an implemented adapter registry.

## Requirement assessment

| Requirement | Assessment and evidence |
| --- | --- |
| FR-002 | PASS at WP04 foundation boundary. Explicit allowlisted refs only; selected committed history and attributed records/links/source blobs survive. Private filesystem state and unrelated refs are not walked. Tests exercise production exporters/readers. |
| FR-003 | PASS at WP04 boundary. Exact object hashes and signed bytes retained; hash/structural/native relationship checks occur in private bare quarantine, with no refs, signer installation or governance adoption. Promotion is deliberately WP05 scope. |
| FR-004 | PASS. Closed issues/comments plus preserved-source records retain retrieved foreign bytes and typed topology, with declared digest-verified attachments; source records remain inert. |
| FR-005 | PASS for allocated snapshot/archive foundation; APIs and values support the dependent local/archive engine. End-to-end destinations/planning are WP05 and CLI WP11 acceptance, not claimed here. |
| FR-014 | PASS. Foreign evidence remains data; selected committed workflows/policy remain historic Git objects, with no execution or active authority adoption. |
| FR-015 | PASS for provider-independent archive/native foundation. Production additions import only standard library plus internal/migration; no provider clients/dependencies introduced. Complete noforge registry qualification belongs to dependent adapter integration. |
| NFR-001 | PASS. Explicit positive record/ref/blob/manifest/Git object counts, individual sizes, aggregate sizes and raw stream framing/wire budgets; incomplete success refused. |
| NFR-002 | PASS. Hostile path/member/type/digest/budget/dependency failures refuse private staging before accepted visibility; source refs stay unchanged. |

## Mandatory anti-pattern checklist

1. Dead code: PASS within the explicitly allocated foundation contract. Targeted call-site search confirms root export calls native staging, staging/archive call canonical validators, and shared helpers are used by these paths. Export/archive read/write are intentionally consumable foundation entry points for WP05; CLI reachability is WP11. Treating the planned boundary itself as a defect would contradict the approved package allocation; no implemented product command is falsely claimed.
2. Synthetic fixtures: PASS. FR checks invoke production WriteArchive/ReadArchive/CanonicalContent/ExportProjectSnapshot/StageProjectSnapshot and real Git/native signed event writers. Literal foreign DTO fixtures are actual input to the production boundary, not constructed expected output.
3. Silent empty returns: PASS. Empty native-root object lists represent foreign-only snapshots; NativeKind empty is an unsupported-ref sentinel rejected by its callers. Other failure paths return explicit errors; cleanup failures do not claim accepted state.
4. FR coverage: PASS for WP04 responsibility as mapped above; later mission-level engine/CLI/provider completion remains separate.
5. Frozen surface: PASS. The entire three-commit diff touches only the six owned files; no existing signed encodings, dependency files or published release/tag artifacts changed.
6. Locked decisions: PASS. No automatic authority adoption/execution, provider dependency, all-ref export, signing-owned stream creation, reset/checkout/force update or source deletion.
7. Shared-file ownership: PASS. Lane metadata assigns the six changed product paths exclusively to WP04. WP05 must coordinate any intentional future adjustment of snapshot.go; WP02 audit protocol integration is the dependent engine's responsibility.
8. Production fragility: PASS. Validation errors deliberately fail loud on unsupported/hostile/incomplete input and retain private cleanup. No transient race is silently converted to success or unsafe publication.

## Integration notes, not WP04 rejection findings

- `Snapshot.GitBundle` names the documented `hn.git-objects/1` sorted raw object stream digest. It is not Git bundle CLI compatibility. Archive v1 is explicitly uncompressed USTAR.
- WP05 must integrate WP02 audit-specific policy/target/rule dependencies. An audit.pack auxiliary snapshot is not a full ancestry root; do not feed such roots to rev-list as if complete ancestry exists.
- Shared StrictJSON intentionally requires all declared fields and rejects null. WP05's optional destination unions/absent create targets require a deliberate validation adjustment with closed-union coverage; current required snapshot/content schemas are internally consistent.
- The baseline pre-review authority reports no_coverage because the generated Python tests.architectural authority is absent. This is documented infrastructure coverage failure, not a Go test failure, and was not bypassed using skip/force.
