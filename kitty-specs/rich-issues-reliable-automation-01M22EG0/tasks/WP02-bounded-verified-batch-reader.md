---
work_package_id: WP02
title: Bounded verified batch reader
dependencies: []
requirement_refs:
- FR-010
- NFR-001
- NFR-002
- C-001
- C-003
planning_base_branch: feat/rich-issues-reliable-automation
merge_target_branch: feat/rich-issues-reliable-automation
branch_strategy: Planning artifacts for this mission were generated on feat/rich-issues-reliable-automation. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into feat/rich-issues-reliable-automation unless the human explicitly redirects the landing branch.
base_branch: kitty/mission-rich-issues-reliable-automation-01M22EG0
base_commit: e3ddc11fc9b9d17609af7a6345862d9f6f5ded55
created_at: '2026-09-09T07:04:06.753083+00:00'
subtasks:
- T005
- T006
- T007
history: []
agent_profile: implementer-ivan
authoritative_surface: issue_read
create_intent:
- issue_read.go
- issue_read_test.go
execution_mode: code_change
owned_files:
- issue_read.go
- issue_read_test.go
role: implementer
tags: []
tracker_refs: []
---

# WP02 — Bounded verified batch reader

## ⚡ Do This First: Load Agent Profile

Use the `/ad-hoc-profile-load` skill to load the agent profile specified in the frontmatter, and behave according to its guidance before parsing the rest of this prompt.

- **Profile**: `implementer-ivan`
- **Role**: `implementer`
- **Agent/tool**: `codex`

If no profile is specified, run `spec-kitty agent profile list` and select the best match for this work package's `task_type` and `authoritative_surface`.

---

## Objective

Build a bounded, verified, in-memory ingestion path for issue queries without a Git subprocess per event object.
Return the same complete validated event facts as the existing loader, together with a token identifying the accepted-ref snapshot actually read.
Keep signed Git objects authoritative and preserve all existing validation semantics.

## Context

Read this mission's `spec.md` and `plan.md`, especially the verified batched reader design and NFR-002.
The current `collectEvents` enumerates actor refs, traverses each history, then invokes Git separately for every event payload and signature.
Issue queries need efficient reconstruction, but caching an unsigned issue projection would introduce an unnecessary trust boundary.
This WP therefore adds a dedicated in-memory reader and leaves unrelated domain callers on `collectEvents`.

The public internal function contract is:

```go
func collectIssueEvents() ([]StoredEvent, string, error)
```

The string is the deterministic snapshot token, not an event ID or execution authorization.
Return no partial trusted result on failure.
WP03 calls this function and translates reader errors into its machine interface.
WP01 independently extends existing event and relationship validators without changing their signatures.
This WP has no dependency on new issue-state types; ingest the existing `StoredEvent` representation.

Start implementation through the mission runtime:

```bash
spec-kitty agent action implement WP02 --agent codex --mission rich-issues-reliable-automation-01M22EG0
```

Use only the runtime-assigned lane/worktree.
Do not edit `event.go`, `store.go`, `quarantine.go`, `shallow.go`, or the CLI wiring owned by other WPs.

### Subtask T005: Bound accepted-ref snapshots and stream Git objects

**Purpose**: Establish a resource-bounded transport from the exact accepted histories to event-object bytes.

**Steps**:

1. Write failing tests in `issue_read_test.go` before the production reader.
   Cover an empty repository, one actor, overlapping local/accepted remote histories, and irrelevant namespaces.
   Verify duplicated reachable commits are loaded once.
   Include malformed local actor ref names and missing ref targets.

2. Create `issue_read.go` with local reader helpers and explicitly named limits.
   The initial ceilings are 100000 events, 8 MiB per object, and 256 MiB aggregate object bytes.
   Count every payload/signature/attachment actually ingested against the aggregate budget.
   Reject excess counts or sizes before allocating their corresponding storage.
   Keep integer conversion and cumulative size arithmetic overflow-safe.

3. Discover the repository once and enumerate accepted refs.
   Follow `collectEvents` local actor fingerprint validation and `parseAcceptedActorRef` selection.
   Ignore proposals, memory streams, quarantine refs, and unrelated `refs/hn/remotes` entries as traversal roots.
   Never select all refs or infer admission from an object merely existing in the object database.
   Bound ref enumeration and parsing as well as subsequent object reads.

4. Sort the selected ref-name/OID pairs and derive the snapshot token from an unambiguous serialization.
   Include ref names as well as OIDs so a change in accepted roots invalidates the snapshot.
   Empty selected roots still have a stable snapshot token.
   Treat ref discovery order as irrelevant.
   Traverse the selected immutable OIDs, not ref names that may move during traversal.

5. Feed selected roots to one `rev-list` traversal and deduplicate commits.
   Stream and bound its output instead of calling an unbounded `Output` helper on hostile history.
   Reject malformed output and fail when the event ceiling is exceeded.
   Use safe Git argument/stdin boundaries; no shell interpolation.
   Handle repository object formats through Git rather than assuming every Git OID is a SHA-1 length.

6. Read `event.json`, `signature`, and required logs using a persistent `git cat-file --batch` process.
   Parse each response header before reading the declared object body.
   Require expected object type and identity/framing; diagnose missing and malformed responses.
   Consume exact body length and framing terminator without a scanner token-size surprise.
   Do not use `git show`, `rev-parse`, or one new `cat-file` process for every event.

7. Ensure pipeline cleanup on success, parse failure, limit rejection, and child failure.
   Close stdin, drain or cancel as appropriate, and reap every started child.
   Bound stderr diagnostics and avoid a writer/reader deadlock from filling pipe buffers.
   Keep batch transport reusable only within this reader; avoid introducing a general command framework.

**Files**:

- `issue_read.go`: new bounded snapshot/traversal/batch helpers, approximately 200–350 lines.
- `issue_read_test.go`: transport and selection tests, sized to the required failure boundaries.

**Validation**:

- Inject truncated headers, oversized declared sizes, missing objects, and truncated bodies through a focused fake-Git fixture.
- Show resource-limit errors occur without materializing the claimed giant body.
- Check subprocess failures terminate promptly and do not leak a running batch process.
- Check identical accepted refs in different enumeration orders produce identical tokens.
- Keep tests isolated in temporary repositories and restore any process-global environment changes.

### Subtask T006: Preserve verification and exact snapshot semantics

**Purpose**: Turn streamed bytes into the same trusted facts as the existing loader, without admitting pending or inconsistent data.

**Steps**:

1. Write failing verification tests using signed fixtures plus deliberate corruptions.
   Include payload alteration, signature alteration/encoding failure, broken actor sequence, broken Previous ID, and unavailable relationship targets.
   Include valid mixed issue, comment, and run-result history where available fixture helpers make it practical.
   Observe actual validation outcomes, not just helper invocation counts.

2. Reuse `verifyEvent` on the original payload bytes and decoded signature.
   Do not marshal-and-verify a reconstructed event or weaken existing protocol/key/content validation.
   Preserve `StoredEvent.ID`, `Commit`, `Event`, `Payload`, and `Signature` values.
   Preserve attachment data exactly; empty attachment maps should match loader semantics.

3. For `run.result`, load `log.txt` through the same bounded batch reader.
   Require its presence and verify `eventID(log)` against the signed `Event.Log`.
   Include attachment bytes in object and aggregate budgets.
   Do not silently omit non-issue records: their actor sequences and relationship evidence matter.

4. Respect the existing replication acceptance state.
   Use `loadReplicationAcceptanceState` or an equivalently faithful read of the existing admission helpers.
   Reject commits denied by pending acceptance, preserving actionable pending error information.
   Fail closed when admission state cannot be read.
   Do not promote, reconcile, repair, or write replication state from this query path.

5. Sort the resulting events using existing timestamp-then-event-ID loader ordering.
   Reuse `validateActorChains` and `validateEventRelationships` after collecting the complete set.
   Timestamp sorting here preserves output compatibility; it must not select an issue revision winner.
   Preserve detailed error causes so WP03 can distinguish invalid history, repository errors, and resource limits.

6. Re-read accepted refs after loading and compare the exact canonical ref snapshot.
   A changed snapshot must return a retryable failure or use a small explicitly bounded retry count.
   Never return the old token with events traversed from moving ref names.
   Re-check pending admission as needed so admission changes cannot create a verified result from denied objects.
   A matching token is an observed snapshot guarantee, not a global transaction or distributed lock.

7. Introduce only the minimal typed/sentinel reader errors needed for stable caller classification.
   A legacy event above a reader budget is a resource limit, not an invalid signature.
   Coordinate exact exported-within-package helper names with WP03 through the root agent.
   Leave the existing general loader and validators unchanged.

**Files**:

- `issue_read.go`: verification, admission checks, stable ordering, and snapshot return path.
- `issue_read_test.go`: tampering, attachments, admission, and race-boundary fixtures.

**Validation**:

- Compare complete `StoredEvent` content with `collectEvents` on admitted valid fixtures.
- Assert corrupted input returns an error and no successful partial event collection/token.
- Use a deterministic Git-wrapper synchronization point to move an accepted ref mid-read.
- Prove the reader reports/retries that change and never silently serves an inconsistent page snapshot.
- Prove unrelated refs do not affect selected event contents or accepted-root snapshot semantics.
- Include a corrupt pending-state fixture and a valid pending object that must remain unavailable.

### Subtask T007: Prove equality and process-count scaling at 1000 issues

**Purpose**: Demonstrate the optimization is real and preserves meaning without imposing a flaky wall-clock threshold.

**Steps**:

1. Add a reproducible fixture with 1000 signed issue openings and realistic title/body content.
   Keep fixture preparation outside timed/counting intervals.
   Use temporary actor keys and repositories; never the developer's configured identity or remote.
   Reuse existing test helpers where appropriate; do not check generated object databases into source.

2. Load the same fixture with `collectEvents` and `collectIssueEvents`.
   Compare full IDs, original payloads, signatures, actors, event contents, attachments, ordering, and total count.
   If comparison needs normalization, explain only representation-equivalent differences explicitly.
   Repeating the new loader on unchanged refs must return identical events and snapshot token.

3. Count Git invocations using a test-only temporary PATH wrapper or another isolated boundary fixture.
   Record total invocations and the commands used for event-object ingestion separately.
   Demonstrate constant batch-object process count at small and 1000-issue sizes.
   Require one history traversal per attempt and no per-event object-fetch processes.
   Ensure the wrapper forwards exact arguments and exit status to the real Git binary without recursion.

4. Report elapsed read duration for both loaders in a benchmark or explicit test log.
   Measure equivalent verification work, not an unsigned projection versus signed reconstruction.
   Do not gate acceptance on a brittle absolute duration or a fixed speedup ratio.
   Process counts and exact equality are the deterministic performance acceptance conditions.

5. Keep scope claims accurate for mixed governance histories.
   Existing relationship validators may separately read proposal policy/pipeline objects.
   Retain those checks; never omit them to obtain an attractive process-count number.
   Report batch event-object scaling separately from existing domain-specific verification reads.
   If this prevents the documented contract, report it to the root agent before broadening ownership.

6. Exercise budget boundaries using lower internal test budgets or bounded fake object streams.
   Cover exact-limit success and one-unit-over-limit failure for count, object size, and aggregate size.
   Avoid allocating hundreds of MiB merely to prove a comparison branch.
   Keep production defaults fixed and documented by named constants.

**Files**:

- `issue_read_test.go`: 1000-issue equality/scaling fixture, benchmark, and budget-boundary coverage.
- `issue_read.go`: only minimal internal seams required for deterministic tests.

**Validation**:

- Run focused reader tests, then the package tests with the race detector.
- Record measured duration, event count, and subprocess counts for review evidence.
- Run `gofmt`, `go vet ./...`, `go build ./...`, and `git diff --check` before review.
- Do not remove a meaningful corruption or admission test to reduce fixture runtime.

## Definition of Done

- `collectIssueEvents() ([]StoredEvent, string, error)` returns verified complete facts plus an exact observed snapshot token.
- New ingestion is bounded before allocation and returns actionable failures without partial trusted data.
- Accepted namespaces, pending admission, signatures, actor chains, relationships, and attachment digests retain their checks.
- The 1000-issue fixture proves semantic equality and constant-count batch event-object processes.
- Existing `collectEvents` callers and durable storage are unchanged.
- Record each completed subtask using `spec-kitty agent tasks mark-status T005 --status done --mission rich-issues-reliable-automation-01M22EG0`, substituting T006 and T007 as appropriate.
- Completion evidence is those event-sourced records, passing checks, and a targeted implementation commit for independent review.

## Risks

- Cat-file framing or pipe cleanup mistakes can hang or allocate excessively: test malformed streams and child exit paths.
- Accepted refs can move during a query: traverse pinned OIDs and compare snapshots before returning.
- Pending admission can deny physically present objects: preserve the existing acceptance-state authority.
- Mixed histories carry policy and CI evidence: reuse validators and report performance scope honestly.
- Tests changing cwd or PATH can race other tests: follow existing isolation patterns and do not parallelize global mutations.

## Reviewer Guidance

Review the batch protocol parser and resource accounting before the happy-path benchmark.
Check every failure path reaps children and returns no partially trusted catalog input.
Compare reader behavior against `collectEvents`, including non-issue evidence and pending replication failures.
Verify count evidence measures the read operation alone and cannot be satisfied by bypassing signature or relationship checks.
Verify the returned snapshot is derived from selected ref names/OIDs and validated after the read.
Confirm only the two owned files changed and there is no new cache, dependency, database, or authority surface.
