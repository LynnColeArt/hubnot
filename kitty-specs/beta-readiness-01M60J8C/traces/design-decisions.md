# Tracer: design-decisions

One entry per finding: `YYYY-MM-DD · actor · <text>`.

---

2026-10-28 · codex · Owner direction and decisions for the beta-readiness mission (2026-10-28):

D01 Execution authority: the owner explicitly grants blanket permission for actions necessary to complete this work and asks that decisions be documented. Continue routine implementation, reviews and qualification without repeated permission questions. Record material scope, authority, external actions, evidence and remaining gaps. Keep test actions bounded and separate from real project content.

D02 Product boundary: preserve native Git-backed operation and independently disabled policy, automerge, audit and contribution features. No mandatory forge, account, Python runtime, database, daemon or Docker. Provider qualification does not make a provider mandatory.

D03 Historical compatibility: retain exact public corpora from released 0.0.1/0.0.2/0.0.3 writers. Tests read frozen fixtures, preserve signed bytes/IDs/refs and exercise continuation. Repair the real memory archive ref-encoding bug with the existing validated helper, without changing signed stream IDs or rewriting old evidence.

D04 Recovery: refuse common actor/memory appends when known accepted same-owner history proves the signing root missing, stale or divergent. Compare signed positions; preserve legitimate older prefixes and independent streams. Restore an older prefix only through explicit verified expected-old Git updates after fencing the prior writer. Preserve true forks for investigation; neither automatically adopt remote signing heads nor claim detection of unknown disconnected signers. Keep pure signature reads separate from ordinary audit snapshot import.

D05 Interrupted operations: validate saved rotation state and signed progress before changing private recovery records. Preserve successful crash retries and uncertainty; never infer remote absence from an empty listing or reset journals to retry a create. Journal schema numbers do not identify the producing release, so retain the originating compatible executable externally. Document existing audit-reader object materialization separately from private/ref mutation guarantees instead of inventing universal transactional rollback.

D06 Contributions: authenticated raw facts remain immutable and policy-neutral. One shared contribution graph/evaluator governs operation authority and admitted views after raw validation. Operative heads/revisions/lineage derive from admissible revisions, not raw rows hidden afterward. Automatic admission runs through bounded durable intake accounting and exact receipts, never by permissive read-time inference. Classify specialized readers/exports explicitly; admission supplies no reviews, CI results, roles, execution or merge authority. This layering legitimately avoids a shared-store edit; any real later need still requires canonical ownership/allocation before editing.

D07 Qualification and reviews: one expensive full suite at a time; use a fixed implementation head and independent review. Metadata-only synchronization preserves validation only after comparing product blobs. Report the framework Python helper no_coverage separately from actual passing Go gates. Real provider/platform/adopter observations remain distinct from fake-server or cross-compilation evidence. Do not mark acceptance from a WP approval alone.

D08 Live GitHub work: the owner first authorized two private LynnColeArt repositories, hubnot-beta-qualification-20261028-source and -destination, with at most two issues, three comments and one edit per repository, migration/retry/conflict checks and retention afterward using the existing GitHub CLI login. Continue within this sufficient bounded plan. CLI identity is LynnColeArt; the connected GitHub plugin identity is different and is not used for these writes. Keep credentials in memory/private process environment, never logs or checkpoints. Qualify only observed API/profile/resource/directions; unavailable profiles stay explicit.

D09 Windows: required future product support, lower priority than current beta recovery. Record XP as a legacy-port feasibility investigation, not a current compatibility promise. Current module requires Go 1.26; official XP support ended at Go 1.10.8 and Git for Windows 2.10.0. Recommend starting qualification with Windows 10 x64 and Windows 11, with final minimum/architectures explicit later. Require real Windows runtime/permission/filesystem/recovery checks; cross-compilation alone is insufficient. Sources and details are in research.md. This does not add a Windows release gate to the current Linux/macOS mission.

Evidence and operational checkpoints: work/beta-readiness-state.md in the task workspace; canonical WP review_result events; per-WP validation logs and private qualification evidence. Update this tracer as significant decisions change.

2026-10-28 · codex · Q01 Available live-provider qualification: mark WP05 T016 complete only for the authorized GitHub REST 2022-11-28 issue/comment cases actually executed. Keep all other provider/profile/resource/direction, cross-provider, rich-resource and injected-response-loss cases explicitly unqualified. A documentation WP's approval is not a blanket beta-support verdict. Final support classification must match observed evidence.

Q02 Preserve hosted test resources for inspection. Completed bounded roundtrip uses source two issues/two comments/one edit and destination one issue/one comment/one edit, inside the approved caps. Completed-session retries and an uncertain changed-baseline resume preserve remote edits and create no duplicates. No budget expansion or cleanup is necessary.

Q03 Treat negative listings as inconclusive. Live GitHub listings returned empty immediately after positive create receipts; retain this observation and verify exact persisted receipt IDs by direct readback. Never repeat a create based only on an empty list. Distinguish this observed behavior from an injected lost-response experiment, which was not performed.

Q04 Keep the WP04 full Go race run at its fixed implementation head. Correct the independent review's transfer-import guide syntax after that run as a separate documentation-only delta; verify Go/module/test-data blobs unchanged and validate actual CLI syntax. A prose correction alone does not justify repeating the expensive full suite.
