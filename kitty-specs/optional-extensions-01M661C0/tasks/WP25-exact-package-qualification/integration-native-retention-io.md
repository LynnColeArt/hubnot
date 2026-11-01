# Native retained-contribution I/O and checkpoint fixture: prospective integration

**External draft only.** No adoption, source edit, claim, gate, task/verdict change or execution result is recorded. Source anchors are H5 `9fc8728896236c9cd341e22edb35ae995143e19d`, tree `b5e5367de767a595750c7822b34fc69a74165615`. Root must review the exact new diagnostic proposals below. The current full-source run must be completely reaped before adoption/reentry. Ongoing native runs keep their actual source identity and continue independently.

## Decision and evidence boundary

The architecture reports identify native per-object Git child amplification: structurally `3O+2R+5` commands inside a successful ReadContributionManifest, plus separate registration closure work and repeated callers. This is source analysis, not measured attribution or a Mac speed guarantee. Historical H3 DarwinARM actual pending16384 took1507.016s/1937.859s and roots timed out later; Intel timed out during actual retained registration setup, before any completed16384 pending observation. The report and root decision retain exact evidence and unknowns. H5 artifact I/O corrections leave these native files unchanged.

The source checkpoint test writes65 files and assumes creation-first is oldest. Production sorts only FileInfo.ModTime, with no public equal-time FIFO promise. The actual failed mtimes/readdir order were lost on cleanup. Explicitly controlled/read-back distinct mtimes repair the fixture assumption; they do not prove the historical cause or authorize a production eviction change.

## Exact scope and ownership

Read the machine-readable regions and verification records. The new sequential exceptions are:

| Existing file and H5 anchor | Allowed work | Original owner retained |
| --- | --- | --- |
| contribution_exchange.go1727–1757 plus adjacent private helper | Native-local bounded closure enumeration/metadata/body/material transport; keep existing preflight semantic call unchanged | beta-readiness WP11 |
| contribution_exchange.go1773–1815 | Retain all current ls-tree/name/order checks; retain OID index and reuse same-invocation preflighted material for payload/signature at original role boundaries | beta-readiness WP11 |
| contribution_moderation.go1521–1548 | Replace only raw closure loop with helper/caller-native adapter; existing preflight and later algorithms unchanged | beta WP12/WP06; optional WP17's separate lock/context seam unchanged |
| extension_changes_test.go117–141 | TEST-ONLY filesystem checkpoint subsection and directly necessary local test helper | optional WP19 |

Only directly necessary gofmt/standard-library import adjustments in those existing files are allowed (for example SHA1/SHA256 hashing or process/callback ownership as actually used); remove an import only after checking all remaining users. No module edit, new Go source path/dependency, blanket file permission or package initialization work. Test-first native transport/public controls use already-owned artifact_read_test.go, artifact_replication_test.go and/or extension_package_acceptance_test.go with explicit separate native role ledgers; this does not grant general artifact production changes. No whole-file transfer, original-owner reopen or ownership-list change. Preserve26 WPs/lanes,107 tasks, T100–T103, WP24 dependency,14 owned paths,13 create_intent entries and all authoritative surfaces/reviews.

## Native-local sequential worker contract

```text
current source/ref/commit checks and existing deadline
  → unchanged bounded rev-list semantics; unique sorted OIDs/count admission
  → worker1 --batch-check: ALL exact OID/kind/canonical-size metadata admission
  → worker1 input close → clean EOF → successful Wait → full cleanup/join
  → worker2 --batch (lazy): one exact OID request/reply at a time
       old framed headroom BEFORE request/allocation
       exact repeated OID/kind/size → exact n+LF → raw Git SHA1/SHA256 hash
       append exact material frame; retain integer offsets only
  → worker2 input close → clean EOF → successful Wait → full cleanup/join
  → existing contributionObjectPreflight
  → unchanged manifest show/strict/schema/namespace/events checks
  → unchanged bounded ls-tree mode/name/cardinality and sorted record checks
  → same-call indexed payload64KiB/signature64-byte role bounds → c.Verify
  → unchanged exact record/project/actor checks
```

At most one worker lives at a time, at most two cat-file workers per nonempty helper invocation. Empty enumeration starts neither. No shared process/cache across calls, actors, plans or phases; keep both independent contributionPlanRetained observations. Retain the constant-cost manifest show before ls-tree. All raw objects remain subject to existing preflight; raw hash does not grant admission/authority or replace signature verification. Offsets, not slices into a growing buffer, identify same-call bounded material. Role bounds remain where roles were originally known; do not impose them prematurely on full closure or move preflight/manifest/tree checks.

Use default classic modes, no --buffer/custom transforms/textconv/filters/mailmap/path/revision inputs. Requests are exact already-enumerated OIDs. Keep projectGit-equivalent scrubbed environment and existing context deadlines. Native stderr is io.Discard, retained storage0: more than8192 bytes alone is valid and cannot refuse. No artifactFailure, artifact stderr policy, artifact8MiB/256MiB constants, --batch-command minimum requirement, global probe/init/registry access, detached worker or host cache.

Metadata completes all per-object/aggregate size admission before ANY body start/request/allocation. The body's repeated header must equal admitted OID/kind/size; changes never enlarge bounds or trigger requery/retry/fetch. Hash exact Git type/size/NUL plus bounded raw bytes before indexing. Missing/changing ODB material fails closed; no snapshot-atomicity or hard Git-internal/kernel allocation guarantee is asserted. Git2.0.5 documentation/source supports historical classic-mode feasibility only, not executed old-version compatibility. Actual qualified Git versions and supported SHA256 repositories need fresh evidence.

## Caller boundaries and exact diagnostic proposals

The implementer must inventory actual pre-edit single-fault native diagnostics and preserve logical old-argument wrappers with honest underlying causes. Do not fabricate ExitError or require identical host OS error spelling. If concrete old behavior cannot be preserved, return the incompatibility to root before expanding policy. Ordered first detection is preserved; arbitrary independent mixed faults have no invented universal priority.

| Existing boundary | Preserved behavior |
| --- | --- |
| bounded rev-list/count/size | Existing enumeration and `Git traversal budget exceeded`/`Git byte budget exceeded`; complete size admission before bodies |
| size-pass process/read | `bounded Git --batch-check=%(objectsize) failed: %w` at equivalent logical legacy boundary |
| reader raw metadata/body | Existing logical metadata/body bounded-Git wrappers, `missing contribution object`, `contribution ref byte limit` |
| registration raw metadata/body | Existing logical metadata/body bounded-Git wrappers, `retained object unavailable`, `retained snapshot byte bound` |
| payload/signature role output | Original logical `bounded Git head:<exact filename> failed: ...` output-budget boundary before c.Verify |
| preflight/manifest/tree/signature/namespace | Existing source calls/checks, ordering and diagnostics unchanged |

The following are **new proposed templates for root/independent review**, not claims of old errors. `%s` denotes bounded exact OID or a fixed native reason, not untrusted raw output; `%w` carries an honest underlying cause. The caller prefix is `contribution` for the reader and `retained` for registration. Retain existing legacy errors whenever a legacy single fault applies; these templates cover only genuinely new protocol/finalization events.

| New native event | Proposed literal template |
| --- | --- |
| malformed/wrong metadata envelope not equivalent to legacy size refusal | `<caller> raw metadata protocol failed for %s: %w` |
| repeated body header/framing/short body/LF violation | `<caller> raw body protocol failed for %s: %w` |
| correct framing but wrong raw Git object identity | `<caller> raw object hash mismatch: %s` |
| metadata trailing stdout/EOF/failed final Wait | `<caller> raw metadata finalization failed: %w` |
| body trailing stdout/EOF/failed final Wait | `<caller> raw body finalization failed: %w` |

Each control pins the adopted literal caller prefix/reason and complete old wrapper where deterministic; OS-specific cause strings stay honestly recorded. Metadata finalization failure blocks worker2 construction; body finalization failure blocks preflight/coverage/success. Preserve the first primary error through cancellation/close/kill/Wait; cleanup-induced cancellation is not parent cancellation.

## Process and transaction ownership

The closure caller is sole owner of current worker Wait, pipe ends and any scoped cancellation callback. Install cleanup on allocation, close every acquired end on partial setup/Start failure, and close parent child-end copies after Start. Context cancellation must unblock owned parent I/O; stop/join any callback before return. On success close input, require clean EOF/successful Wait and join cleanup before next phase/material return. On failure cancel/close/kill as needed, Wait exactly once and retain first error. No finalization error may disappear in a successful defer. Stage witnesses in tests must identify exact PID/PPID/session/role and be observed predeadline while candidate/peer live, with observers joined.

Registration retains both plan recaptures, selected authority/heads/native roots, existing x.lock/command lock, Reserve/save/descriptor-before-read order, charge/accounting/coverage/journal/ref/settlement algorithms. A failure after reservation may retain the existing permitted charged reservation/descriptor; do not incorrectly require all private state unchanged. Require no unauthorized coverage/ref/admitted visibility and exact documented permitted accounting effects.

## Deterministic checkpoint fixture

Preserve source test lines101–116 (constructed200000-ref accounting,128MiB proof plus16MiB capture within256MiB) and142–156 (17×16MiB memory eviction) byte-identical. Change only117–141 plus a directly necessary local test helper/import. Set explicit widely separated valid filesystem mtimes after each write and read them back before the65th insertion can prune; do not sleep or change production clocks. Use fresh independent states for normal creation-order and intentionally different creation-vs-mtime-order scenarios. Assert exactly which minimum mtime is evicted, newly written/current protected descriptor stays valid,64 actual descriptor filenames plus exact key/lock entries remain, and other intended descriptors remain. A negative/alternate-order control must fail if the test silently resumes creation-order/FIFO assumptions; no equal-time FIFO requirement. Do not rewrite immutable descriptor contents or reset globals beyond the unchanged existing RAM test.

## Sequencing and qualification

1. Preserve fully reaped H5 local failure/run and all317 identities. Native H5 jobs retain source binding and continue; do not cancel/relabel them.
2. Root reviews this exact external amendment/diagnostics and adopts normal metadata only, preserving user dirty projections and all canonical approvals/lanes/tasks. Normal validate/finalizer planning refresh, fresh actual analysis/record-analysis and normal WP25 implementation reentry precede code authorization.
3. Author writes meaningful test-first native/public/fixture controls, then only scoped corrections. Preserve original failures and exact old/new source/fixture/binary/Git identities. No new tasks, sample cuts, budgets, caps, timeouts or feature semantics.
4. Independent source/error/lifecycle review checks all317, unchanged protected ranges/callers, scoped ownership and full-return finalization. Root issues new311 attestation only on approved frozen clean source.
5. Fresh unchanged default/detector/history controls and complete4320 observations/2160 pairs/72 groups/144 elapsed+RSS criteria; full frozen uncached race/static/build/format; all8 actual Linux/macOS AMD64/ARM64×standard/combined-native package rows and12 supplied references. Keep exact failures/pendings, no source-rebuild fallback or historical reuse.
6. Normal WP25 formal review/approval, then WP26 independent cold trials/readiness/all52 trace, canonical accept and feature-target merge/post-review/retrospective. No beta/main/release change.

The separately observed Intel migration plan45-second watchdog expired after Preview before Saved plan, with no exact later operation retained. This native reader is not on that selected path; this amendment neither diagnoses nor fixes it. Keep the watchdog and fresh actual migration journey mandatory. Any remaining failure receives its own evidence/scoped owner decision.
