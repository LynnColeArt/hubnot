# Independent WP06 review

Verdict: APPROVED after pre-handoff correction
Reviewer: codex-independent-interop-review, resolved reviewer-renata; no implementation edits
Initial reviewed source: ad936d0f6efd59134ab3f7f0ddf1ddbc8d0c3177
Final reviewed source: 8c2c187884f5055375af146d9fb40ddf4070cf13
Canonical review invocation: 702a9d08b0a4450ea2cbcd4a48ae8fcb
Scope: registered resource kinds, strict typed states, revision engine, native CLI registration, narrow issue-operation guard, store/quarantine/shallow closure and protocol/resource documentation.

## Findings and dispositions
R1 (P2), accepted and fixed before review handoff: initial replicationEventDependency checked direct refs and then traversed deeper reply nesting through structural validation. With thread and immediate parent present but grandparent supplier absent, an independent probe returned a structural error with no dependency record, incorrectly labeling valid incomplete history as relationship-bad. It failed closed, so no unsafe promotion occurred. Fix 8c2c1878 distinguishes resourceDependencyMissing and maps it to exact dependency-missing/recovery before promotion; a present wrong-kind ancestor remains invalid. Independent original reproduction now passes under race.

Reviewed the implementer's strict-union follow-up 9bdc7aad: raw empty inapplicable target/value/field members now reject before DTO decoding. This preserves the documented typed union rather than relying on nonzero Go values. No remaining actionable finding.

## Acceptance assessment
Five independent signed aggregate families are present: project, project-item, discussion, discussion-reply and workflow. New kinds use existing Event Body/Subject/Parents/Intent/Operation/Request and actor histories; no IssueState flattening or provider dependency. Strict new-kind raw checks reject malformed UTF-8/surrogates, duplicates, unknown/case-aliased fields and oversize authenticated payloads. Canonical type-specific state enforces identifiers, collections, decimals/dates, workflow pipeline/source digests and unrelated authority-field restrictions. Workflow creation never executes or materializes foreign content.

Independent exact-reference probes confirm issue targets reject issue.revise/comment, discussion targets reject discussion.revise/reply, schemas from another ProjectID reject, and accepted reply revisions from another thread reject. Reply container/nesting parent immutability and depth, exact schema pinning and answer revision pinning are enforced. Topological revision projection retains all maximal heads with null conflicted state. Mutations pin the actor, guard exact head sets and preserve original operation intent on replay; issue/resource key collisions refuse and lookup does not expose another family's operation.

All admission seams were reviewed: verifyEvent/content, explicit issue validator carveout, store relationships, quarantine dependencies/reference extraction and final pre-promotion graph validation, shallow closure. Resource facts use existing selected actor refs. Existing bounded reader remains 100k events/refs, 8MiB objects and 256MiB aggregate. Real bare-remote/fresh-clone tests reconstruct all five families, quarantine missing suppliers until selected, and preserve old signed payload/signature bytes. Older binaries' unknown-kind refusal is documented explicitly; old encoders/signatures/authority are unchanged.

## Verification
- Independent original TestResource* run: PASS (0.977s).
- Independent missing-nested-ancestor reproduction at initial source: FAIL as expected (dep=nil plus structural unavailable-resource error).
- Independent actor pin, cross-issue operation collision, null/typed input and exact-reference probes: PASS at initial/strict-union heads (0.062s / 0.065s).
- Final independent full resource overlay under race, including original missing-ancestor test: PASS (1.103s).
- Exact-final-head implementer full race accepted by orchestrator: hubnot PASS 233.922s; internal/forge PASS 1.011s; internal/githubapi PASS 11.250s. Focused/tagged tests, vet, default/nogithub build, formatting and diff checks PASS.
- Default for_review gate completed without force/skip; generic no-coverage/JUnit baseline is unavailable rather than a Go regression.
- Fresh canonical prompt read and final source rechecked at 8c2c1878; no differences in reviewed files.

## Required anti-pattern checklist
1. Dead code: PASS for live engine/command/admission paths: native localCommands registrations reach CmdResource and mutation/query/projection validators; store, quarantine and shallow processing invoke resource validators/reference extraction. Typed Go helper APIs ResourceStateJSON/DecodeResourceState are explicit seams for dependent family WPs, not separately claimed user features.
2. Synthetic-fixture tests: PASS. Model fixtures feed actual validators/projections; actor pin/replay/conflict and real bare-remote/clone tests exercise signed native storage and admission. Tests do not substitute expected response literals for production behavior.
3. Silent empty return: PASS. Nonresource reference extraction and empty native collections have documented meaning. Missing/wrong dependencies, invalid state, stale guards and operation collisions fail explicitly.
4. FR coverage: PASS for the native foundations of FR-013/014/015/016, bounded hostile input and independence. Specialized family UX/materialization remains subsequent WPs.
5. Frozen surface: PASS. Existing signed payloads/signatures and old encoding semantics remain byte-identical; no frozen evidence rewritten.
6. Locked decisions: PASS. New kinds are explicitly scoped; no identity/policy/CI authority or mandatory provider/service dependency; foreign workflow source inert; no timestamp conflict winner.
7. Shared-file ownership: PASS. All core Event/store/quarantine/shallow/model changes are contained in WP06. issue_commands.go narrow cross-kind lookup guard is explicitly authorized. Generic local dispatch uses WP03's existing extension seam without modifying main.go.
8. Production fragility: PASS. Invalid versus recoverable missing input is distinguished; explicit stale/conflict/retry errors replace silent acceptance. No swallowed validation or execution fallback found.

## Integration notes
NativeResourceCommands test cleanup currently deletes its temporary project registration; when WP07 installs the real project specialization, that test must restore the previous map value as already assigned by the orchestrator. This is not a current core production defect. Later WPs must retain exact revision guards, documented authority limits, native workflow materialization/run binding and provider-optional acceptance. This approval does not declare the full expanded mission complete.
