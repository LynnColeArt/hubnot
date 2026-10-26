# WP04 → WP05 sequential bridge maintenance concurrence

2026-10-26. Recorded by the WP04 implementation owner after reading
`work/wp05-integration-advice.md`, the approved WP04 bridge at
`41dc551c4d84624334b4d48bed994410129601bf`, WP05's declared surfaces, and the
canonical `ownership-map-leeway` tactic. This is coordination advice and owner
concurrence, not implementation, a review verdict, another approval, or a WP
lifecycle change. No product tests were run for this advisory.

I concur with a narrow, sequential handoff of maintenance of the existing
`project_snapshot.go` to WP05 for the two combined WP02/WP04 integration defects.
The original WP04 implementation correctly used the then-read-only native loader;
WP02's audit loader now imports auxiliary objects into its supplied object DB.
Consequently, the existing export call through `projectDependencyRoots` can
write source objects, and the current staging order can run full Git fsck after
snapshot-only audit objects have been unpacked. Both need correction at this
canonical bridge, rather than a parallel exporter or admission implementation.

The handoff starts only when corrected WP02 is approved, WP04 remains frozen,
and root dispatches WP05 with this exact adjacent-file maintenance scope in its
runtime prompt. Root retains ownership/lifecycle coordination; this note does
not edit any ownership/frontmatter/status authority. There must be no concurrent
WP04 edits, and no speculative rebuild of the WP05 lane before its dependency
base is ready. WP05 should record the one-line rationale in its scoped commit:
"Maintain the canonical project snapshot bridge for read-only export and bounded
audit auxiliary admission after approved WP02/WP04 integration."

The additional editable product surface is `project_snapshot.go` only. Preserve
its exported signatures and the archive/raw Git material formats. Keep
`internal/migration/snapshot.go`, `archive.go`, `archive_test.go`,
`project_snapshot_test.go`, and `docs/project-migration.md` frozen. Put the new
integration regressions in WP05-owned `migration_native_test.go` (and existing
WP05 engine/local/journal tests as appropriate), and explain the final integration
behavior in WP05-owned `docs/migration-engine.md`. This is not authorization to
edit WP02's canonical audit validators. Recheck their approved signatures and
side effects before implementing; the advisory's active corrective WP02 checkout
is not a dependency pin.

Concrete bridge maintenance is bounded to these responsibilities:

- Export discovers signed legacy dependencies in disposable private inspection
  staging, never by invoking an importing native reader against the source DB.
  Copy the explicitly selected bounded ordinary closure first; preflight audit
  expansion there before canonical native readers unpack it; read subsequently
  identified legacy code dependencies from the source without mutating it.
- Stage verifies raw material hashes and limits and runs ordinary full-history
  structural checking before any audit reader imports auxiliary objects. Keep
  ordinary selected refs and legacy code/memory dependencies as full closure.
  Audit targets/policy/source path proofs retain the exact signed pack closure;
  omitted parents and unrelated proof siblings are not required as ancestry.
- Use approved canonical pack measurement, binding, object-shape, signature,
  actor-chain, policy-role and relationship validators. Aggregate saved migration
  Git limits over the union of ordinary and expanded audit object IDs, counting
  shared IDs once, before unpacking. An individual pack limit or objects already
  present in the destination cannot substitute for that migration budget.
  Recheck the union when discovery adds ordinary legacy dependencies.
- Compare supplied raw IDs with ordinary closure independently of auxiliary
  unpacking. Retain exact actor trees and raw `audit.pack` bytes in the archive.
  Expose the complete admitted object-ID union through existing `ObjectIDs()`;
  keep `Roots()` restricted to roots safe for full-history traversal.
- WP05's own native apply boundary copies the admitted union or reinstalls
  already validated exact packs before expected-old observation-ref visibility.
  It must not send incomplete audit roots through `pack-objects --revs` or rely
  on a later accepted read to materialize missing objects. Keep destination
  identity, worktree, active governance and private audit-authority anchors intact.

I also concur with leaving WP04's `StrictJSON` unchanged. WP05 can reuse its
lexical protections through generic-map decoding, then close every optional
write-union level in its sole `plan.go` validator. Presence-aware exclusivity and
action checks remain necessary: absent create target, required update target,
exactly one RemoteRef branch, bounded nonempty literal ID, and closed declared
receipt references. Do not duplicate Unicode/token logic or loosen the mandatory
source issue/comment schemas.

Required regression evidence belongs to WP05's independent acceptance:

- Successful offline export/archive/stage/apply from audit-only and keyless
  accepted-observation histories whose valid packs omit parents/proof siblings;
  later audit reads work, with exact signed payload/signature/pack bytes.
- Before/after source refs, worktree/config and object DB remain unchanged even
  when audit dependencies exist only inside the signed pack initially. Compare
  the source object set and object-store files, not refs alone.
- Wrong or missing proofs/edge types, digest/binding/log/signature/roles and
  missing signed supplier facts refuse before accepted visibility; aggregate
  expanded-object limits fail before unpacking, with shared-ID accounting.
- Ordinary incomplete selected code ancestry still refuses; stale destination
  and lost-ack ref recovery retain guarded journal semantics; no local signer,
  current audit anchor, implicit policy adoption or execution is introduced.
- Optional union positive/negative cases pass while all prior unknown, duplicate,
  null, invalid-Unicode and missing-required source-schema refusals remain.

Run focused regressions and the required fixed-head Go race/vet/build/format/diff
checks, then independent review through the canonical lifecycle. WP04's prior
approval is neither reopened by this note nor evidence for the new combined
behavior. No source, commit, test, lane, status, or approval mutation was performed
in this coordination response; the only authored artifact is this scratch note.
