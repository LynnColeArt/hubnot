# Independent WP11 review

Verdict: APPROVED after pre-handoff corrections; no remaining actionable finding.
Reviewer: codex-independent-interop-review, resolved reviewer-renata; no implementation edits.
Original reviewed source: 74002d4e4f2e19ac3017458f5c63d6d053fed745.
Final reviewed source: 26e4c55474414decd979c3f100a41f889fae3ea9.
Canonical invocation: c80367b236254a7086c72ff521b75b07.
Scope: github_discussions.go and github_discussions_test.go. SDK/neutral bridge/native dependencies were independently approved; their contracts were read where needed. This reviewer authored the SDK dependency, not this adapter. No contracts/ artifact exists for this mission.

## Findings, corrections and red/green evidence

### R1 — P2: checkpoint progress could redirect a child to an unrelated parent

At74002d4e, githubDiscussionCheckActions validated the signed native content but accepted editable progress/bindings. After exporting a native root plus parent/child with a lost parent response, the independent probe changed Next from1 to2, cleared Pending, changed the native parent's remote binding to an existing unrelated HUMAN comment, and removed the parent baseline. Resume sent one child mutation with ParentID=HUMAN and returned success. A missing parent baseline and unauthenticated completed-step progress could therefore manufacture association authority.

Independent TestIndependentDiscussionRejectsManufacturedParentBinding was RED at74002d4e: FAIL0.194s, writes=1, err=nil, parent=HUMAN. Root held full gates and explicitly authorized the bounded adapter-local private signature refinement. No canonical rejection was required because this finding occurred while implementation remained in_progress.

Fix26e4c554 adds a domain-separated Ed25519 signature over the complete canonical checkpoint, including scope, native frozen/previous snapshots, stable source/destination repository IDs, category, actions, progress, bindings, baselines, pending intent and results. Every real export loads and validates the currently selected native identity. Every load checks canonical raw JSON, signer/public-key fingerprint, selected identity and full signature before adoption; every save re-signs the complete state. Identity changes explicitly refuse recovery. The complete signature envelope participates in worst-case preflight. Private keys are never serialized into checkpoint data. Preview remains private-key-free and cannot issue writes. Signed native snapshot/source witnesses, guarded baselines and positive remote reconciliation still apply: a journal signature does not establish remote success or grant provider permissions.

The unchanged independent probe is GREEN at26e4c554 with zero additional provider writes. TestGitHubDiscussionRegressionRejectsManufacturedParentBinding permanently retains the concrete scenario in production tests. Additional public-routing tests reject unsigned/noncanonical/aliased/signature-altered journals and an active identity switch, while hidden-keyring preview succeeds and real export without a signing identity refuses.

### R2 — P2: repository rename could manufacture new-copy mode

At74002d4e, githubDiscussionSource compared the mutable source.Project selector with the selected owner/name. The independent probe imported immutable discussion D1 from o/r, then changed only the selected repository name to o/renamed while the provider still returned immutable repository R1/discussion D1 at the same endpoint. Same-source became false, ordinary imported content had no native publication marker, and export sent CreateDiscussion rather than preserving the unchanged source.

Independent TestIndependentDiscussionSourceRepoRenameRetainsIdentity was RED at74002d4e: FAIL0.170s, roots=1, writes=1, err=nil.

Fix26e4c554 resolves the signed immutable source discussion globally through GetDiscussionByID, compares verified source/destination repository IDs, and treats owner/name as a lookup hint. Inaccessible source identity refuses before creating a copy. Existing signed journal repository identities are revalidated, and pre-write checks refuse a discussion moved to another repository. A genuinely distinct destination repository still supports an explicit copy.

The unchanged independent rename probe is GREEN at26e4c554 with zero provider mutations. TestGitHubDiscussionRegressionSourceRepoRenameRetainsIdentity retains it in production. TestGitHubDiscussionDistinctStableRepositoryCreatesIntentionalCopy proves one copy to a different stable repository, followed by a no-write repeat.

## Acceptance assessment

The optional adapter is installed through the real provider resource registry and ReadDiscussions interface, using the approved fixed-document SDK. Both new files use !nogithub. Public hn transfer routing, selected numeric discussion lookup and bounded repository listing/hydration feed a complete typed graph before native application. Original source IDs, URL/author/timestamps and category/lock/poll/reaction/minimization/edit-count observations remain descriptive source claims; local native actors sign imported facts. Independent replies retain parent and accepted-answer references, and graph/native validation rejects inaccessible or cross-thread references. Native duplicate-publication refusal verifies exact signed native revision content, even without the private export journal.

Import explicitly requires history=snapshot; unsupported source semantics require losses=acknowledge and are retained/reported as descriptive claims rather than moderation authority. Export requires an accessible explicit category ID and verifies answerability. Unsupported deeper nesting and historical accepted-answer content differing from the current mutable reply explicitly refuse before any write, rather than silently reassign the answer to different content. Native and external competing edits refuse overwrites. Verified same-source returns preserve ordinary source bodies, update replies in place, suppress footer echo, and never resurrect deleted unchanged source replies. New destinations receive visible native attribution and strict terminal structured proof markers; ordinary marker-like prose remains content.

Whole-export preflight accounts for the frozen/previous graph, every action and future binding/baseline, the largest pending/result representation, escaped1024-byte opaque IDs, and signature envelope within the8MiB private journal limit. Predictable overflow fails before provider mutation. Each actual remote step uses the shared prepared/in_flight/confirmed executor and one SDK mutation. Unknown creation/reply outcomes require exact positive content/identity reconciliation; a complete negative scan does not authorize another POST. Checkpoints authenticate selected native snapshots and producer-shaped imported baselines throughout resume. External attribution and private journal signatures grant no roles, policies or provider permissions.

## Verification at exact final source

The final lane HEAD was verified26e4c554 before/after independent probes and again during canonical review. No tracked source changes or reviewer production edits occurred; runtime-generated untracked .spec-kitty state was left untouched.

- Unchanged original independent overlay: go test -race -overlay discussions-adapter-overlay.json -run '^TestIndependentDiscussion' -count=1 . — PASS3.496s. Both original REDs now pass. Additional probes verify frozen-snapshot/source-mode tampering refuses before writes, uncertain reply negative scan never retries, same-source edited reply updates/repeats without duplication, and native publication re-import refuses after journal removal.
- Focused correction/public-routing tests: go test -race -run '^TestGitHubDiscussion(RealExportIdentityChangeAndKeyFreePreview|CanonicalCheckpointRejectsUnsignedAndAliases|DistinctStableRepositoryCreatesIntentionalCopy|Regression)' -count=1 . — PASS2.457s.
- Author complete discussion-focused race suite: PASS11.480s, supplied by orchestrator. Original pre-handoff suite PASS7.533s did not override the two independent RED findings.
- Final exact-source root gate manifest wp11-26e4c554-gates.json records source26e4c554, lane-k, command, log and exit_code0 for both full suites. Full go test -race -count=1 ./...: hubnot298.451s, forge1.016s, githubapi10.243s; wrapper299.124s. Full go test -tags nogithub -count=1 ./...: hubnot150.170s, forge0.002s; wrapper152.701s. These exact logs were read for this canonical verdict; duplicate full suites were unnecessary.
- Vet, default/nogithub builds, formatting and source diff checks passed for the author and were rechecked by this reviewer. Tagged dependency exclusion passed as reported by the orchestrator; the full tagged suite contains no githubapi package.
- All15 SDK documents and its hostile transport/pagination behavior are the independently approved WP14 dependency and were unchanged by WP11. No npm/parser or external Go dependency enters the adapter.
- Default canonical for_review regression gate ran to completion and returned known no_coverage because tests.architectural._gate_coverage is unavailable; no force/skip bypass. The baseline JUnit/Python authority failure is not an actual Go regression.

Outside probe evidence remains at /home/lynn/Documents/Codex/2026-10-24/wa/work/interop-review/discussions_adapter_review_test.go and discussions-adapter-overlay.json; final gate manifest/logs remain in the same work directory. Durable final regressions and this source-bound report retain both original findings and corrections. No live provider writes were made.

## Required anti-pattern checklist

1. Dead code: PASS. init installs the export callback/capability; the neutral production reader invokes githubForge.ReadDiscussions, and actual public run/transfer tests exercise both import and export. Internal planning, proof, baseline, budget, signature and recovery helpers have live callers.
2. Synthetic-fixture tests: PASS. Local GraphQL fixtures invoke real Client parsing, the public transfer route and actual signed native resource storage. Independent probes alter real private journals and inspect actual provider mutation counts/parent identity; deleting the adapter would fail their behavior assertions.
3. Silent empty return: PASS. Absent source association denotes a native/cross-provider resource, absent optional refs are explicit, and verified unchanged/exported source data intentionally produces no mutation. Missing witnesses, inaccessible identities, malformed state/journals, unsupported mappings and conflicts return explicit errors; no timestamp winner or empty fallback hides a failure.
4. FR coverage: PASS. FR-004 source attribution/claims and independent replies; FR-008 explicit optional transport/token and no-key preview; FR-011 neutral identity/graph/registry boundary; FR-012 capability/mapping/loss reports; FR-013 real native thread/reply/answer creation through import; FR-017 full discussion import/export/repeat/recovery/category/answer behavior; FR-020 complete typed graph and pre-write mapping/budget/refusal paths all have production-path assertions. Core native semantics and SDK transport are separately approved dependencies.
5. Frozen surface: PASS. The two WP commits touch only the two owned new optional adapter files. SDK, Event/native signatures, existing signed data and dependency source remain unchanged.
6. Locked decision: PASS. No mandatory GitHub/gh/service/Docker, implicit background sync, imported permissions/moderation authority, foreign execution, blind negative-scan retry or exactly-once promise. The explicit real-export identity requirement and key-free dry-run match the amended WP recovery contract.
7. Shared-file ownership: PASS. No shared source file is modified by this WP. Registry installation is through the approved existing optional extension seam; no SDK/core/bridge edit is hidden in this diff.
8. Production fragility: PASS after R1/R2. Full journal/signature bounds fail before writes, malformed recovery state cannot redirect publication, renamed selectors cannot manufacture copies, uncertain outcomes refuse unsafe retries, and errors preserve source/remote edits for explicit reconciliation. No panic or automatic authority fallback is introduced.

Approval covers WP11's documented supported adapter subset. Whole-mission acceptance/documentation and the other resource adapters remain separate required work.
