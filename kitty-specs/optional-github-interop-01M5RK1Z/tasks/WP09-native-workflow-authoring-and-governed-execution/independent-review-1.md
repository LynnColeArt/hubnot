# Independent WP09 review

Verdict: APPROVED
Reviewer: codex-independent-interop-review, resolved reviewer-renata; no implementation edits
Reviewed source: 16e230402bb8490a5e62ed4ec1f1ba4d630fe8a3
Canonical review invocation: 2879a1ba18f14dc1a924e5dae6254879
Scope: workflow_commands.go and workflow_commands_test.go.

## Assessment
No actionable defect found in full source audit. Native workflow handler registration preserves generic strict signed authoring/revision/resolution/list/history/operation commands. Enriched show distinguishes runnable native, disabled, missing definition, inert foreign source and conflict. Selection requires a sole current head and any supplied exact expectation. Pipeline bytes come from validated hn.pipeline/0 and deterministic compact JSON; foreign source documents remain descriptive inert state.

Materialization requires a Git worktree, opens the repository and each real directory through os.Root, rejects symlink components and checks opened/current file identity. Bounded destination reads reject nonregular/symlink substitutions. A random O_EXCL temporary file receives complete canonical bytes and fsync before publication. Atomic link publication does not replace a competing existing destination unless --overwrite was explicit; overwrite replaces the directory entry, preserving other hardlinks' contents. Existing identical files are a no-op. Invalid native selection fails before directories are created. Materialize requires no signing key, creates no commit/event, and never executes commands or contacts a provider.

Run requests retain the ordinary cmdRunRequest guards: shallow preparation/closure, exact candidate resolution, proposal dependencies, base policy, signed proposal code-ref match and pipeline definition. The immutable proposal pipeline bytes and digest must exactly equal the selected signed workflow's canonical pipeline. Active actor/optional actor pin then produces an ordinary run.request with existing Subject/Pipeline/Definition/Commit fields. Workflow revision is reported in output, without misusing signed evidence or adding authority/schema fields. This explicitly invoked request does not execute a pipeline; ordinary runner policy/isolation and ordinary request semantics remain unchanged.

## Verification
- Independent submitted TestWorkflow race suite PASS (3.914s): canonical/inert materialization, differing-file refusal and explicit overwrite, directory/file/internal symlink refusal, hardlink target preservation, disabled/foreign/conflict refusal, generic authoring/replay/resolution, successful governed exact request and mismatched bytes/policy/code-ref/actor refusal. Public nogithub binary dispatch is exercised.
- Additional independent overlay race PASS (1.239s): competing no-overwrite publications produce exactly one complete winner without temporary remnants; request binds immutable proposal bytes despite hostile uncommitted working-tree contents; subdirectory materialization writes canonical repo-root bytes after all signing keys are removed; unsigned request refuses with unchanged refs.
- Independently reviewed exact source, helper caller paths, comparison with cmdRunRequest, and diff check. No provider dependency, execution, automatic commit, policy/runner changes or new Event fields. Source rechecked unchanged during canonical review.
- Inspected root's exact-source full race log: hubnot PASS 253.093s, internal/forge PASS 1.016s, internal/githubapi PASS 10.250s. Supplied focused race 3.484s, tagged tests 1.255s, vet, both builds, formatting and diff pass. Default for_review gate completed exit0 without bypass; unavailable generic Python/JUnit coverage is not a Go regression.

## Required anti-pattern checklist
1. Dead code: PASS. Registry reaches cmdWorkflow, parsing/rendering/show/selection/materialization/request helpers; filesystem helpers are called by materialization and canonical definition helpers by show/materialize/request. All new production helpers have live callers.
2. Synthetic-fixture tests: PASS. Fixtures exercise actual dispatcher, signed engine/storage, real filesystem and Git proposal commits; the public test builds/runs the transport-excluded executable. Assertions inspect real files, refs and run.request facts.
3. Silent empty return: PASS. Identical destination bytes intentionally return changed=false; missing files are distinguished from errors; conflicted show deliberately has no runnable state. Validation/publication/guard failures propagate; no execution fallback.
4. FR coverage: PASS. FR-015 has signed native definition/status, explicit materialization and exact governed request evidence. FR-016 uses approved native signed guards/replay/conflicts with generic authoring/resolution, preserved source/metadata and offline operation. Admission/replication remain unchanged from WP06.
5. Frozen surface: PASS. Exact commit adds only owned command/test files. Old signatures, Event schema, pipeline/runner authority and frozen evidence unchanged.
6. Locked decision: PASS. No implicit foreign execution, network/provider requirement, automatic Git commit, timestamp winner, new runner privilege or undocumented signed authority field.
7. Shared-file ownership: PASS. No shared production/test file modified; registry and approved core APIs are reused.
8. Production fragility: PASS. Atomic no-replace publication, explicit overwrite and stale/conflict/policy/code-definition checks fail clearly; no panic, silent partial canonical bytes or hidden retry/execution is introduced.

Approval covers native workflow commands only; foreign workflow conversion and external writes belong later adapters. The expanded mission is not yet complete.
