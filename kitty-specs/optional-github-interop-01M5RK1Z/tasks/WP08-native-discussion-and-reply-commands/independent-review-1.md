# Independent WP08 review

Verdict: APPROVED
Reviewer: codex-independent-interop-review, resolved reviewer-renata; no implementation edits
Reviewed source: 2882cc1dfee9ca9696e6749e3bc66c7c2637bb14
Canonical review invocation: 45192103976b47c5810bd35aaada6ad3
Scope: discussion_commands.go and discussion_commands_test.go.

## Assessment
No actionable defect found. Native discussion and reply specializations register through the existing core command registry. Ergonomic thread open/revise/close/reopen/category and reply add/edit operations preserve the generic strict full-state open/revise/resolve, list/heads/history/operation surface. Convenience changes derive from the explicitly expected historical state, preserving unrelated metadata and reconstructing immutable operation inputs after later edits. All writes use core actor pinning, exact expectations, actor-global operation identity and explicit all-head conflict resolution.

Reply roots and revisions remain independent signed resources. Core references enforce same-thread parents, immutable containers/parent, depth <=8 and exact answer revisions. Enriched discussion show returns the signed actor and body from the selected answer revision even after later reply edits; concurrent thread state remains null and does not invent an answer winner. Reply counts include independent roots, and bounded reply pages retain conflict heads, exact attribution and accepted-ref snapshot/query scope. Reads and help need no active private key. Category/status changes are explicitly descriptive and introduce no imported moderation authority, provider dependency or execution path. Generic admission/quarantine/shallow semantics are inherited unchanged from approved WP06; earlier independent missing-ancestor/reference probes and core replication gates remain applicable. Mission-wide documentation/acceptance remains WP05.

## Verification
- Independently ran all TestDiscussion tests under race at exact source: PASS (3.157s). These exercise actual run(), core signatures/storage, reply nesting and cross-thread refusal, thread/reply conflicts and resolution, old answer content, lifecycle replay, operation lookups, metadata preservation, bounds and reads after removing signing identity.
- Additional independent overlay under race: PASS (1.634s). Replaying an old answer operation after reply edit, close and clear leaves the current closed/cleared state and metadata intact; accepted content before clear remains the original exact revision; --clear=false refuses with unchanged refs. Reply cursors reject another thread and changed snapshots; literal body --input remains ordinary content.
- Independently inspected exact source, live helper callers, bounded typed core paths and diff check. No provider/network/process import or panic path introduced. Source remains 2882cc1d; no production edits.
- Orchestrator supplied exact-source full race PASS (278.695s), final full nogithub suite PASS (156.326s), vet, default/nogithub builds, formatting, diff and provider-dependency exclusion. Default generic for_review gate completed no_coverage without force/skip; generic unavailable Python/JUnit coverage is not a Go regression.

## Required anti-pattern checklist
1. Dead code: PASS. init registers cmdDiscussion/cmdDiscussionReply; these call discussionHasInput, discussionEdit, discussionRead and generic CmdResource. Both mutation/read paths call discussionResult, and edits call discussionBase. All new production helpers have live callers.
2. Synthetic-fixture tests: PASS. Typed fixtures feed production commands/engine and signed native storage; assertions inspect resulting bodies/heads/replay/ref changes. Conflicts use valid captured actor histories, not response literals.
3. Silent empty return: PASS. Help returns success intentionally; absent accepted_answer means no selected answer or a conflicted thread. Empty reply pages are normal collections. Validation/read/mutation errors propagate, and internal unmarshals consume previously validated states.
4. FR coverage: PASS. FR-013 has independent signed threads/replies, categories/status, exact answers and queries; FR-016 has immutable guarded replay, actor mismatch refusal, visible concurrent heads/resolution, preserved metadata and offline/native evidence, using approved core replication/admission.
5. Frozen surface: PASS. Exact commit adds only the two owned command/test files; old signed bytes, Event shape, policy and frozen evidence remain unchanged.
6. Locked decision: PASS. No mandatory provider/service/gh dependency, moderator ACL, authority import, moving accepted-answer lookup or timestamp conflict winner.
7. Shared-file ownership: PASS. No shared file is modified; handler registration uses the approved registry.
8. Production fragility: PASS. Invalid flags/targets, stale guards, conflicts and wrong actor fail explicitly. Historical input reconstruction preserves safe operation retries. No panic, execution fallback or swallowed validation is introduced.

This work-package approval does not approve the later external discussion adapter or declare the expanded mission complete.
