# WP02 independent corrective review

Reviewed source: `6d687af50562e8d837915d39a97d82727e91fd71`.
Tree: `c38c1bf8d5b2cd3534bd1c44fde2832e68c38409`.
Runtime review invocation: `6f8c400c251e4db4a403c1d6dff0b7a5`.
Recommendation: approved; original P1 is resolved. The canonical runtime event remains the verdict authority.

The full new runtime prompt, prior feedback, corrective diff, production call sites, permanent regressions and source-bound verification were inspected. Repository AGENTS/charter and mission spec/plan/research/data-model/quickstart/audit contract were read in the first cycle and remain the review contract. No product edits were made during either review.

## Finding disposition

Both local writes and imported reads now run `importAuditPack`, which validates hostile packs in a fresh isolated object database before copying to the selected database. `index-pack --fsck-objects` rejects malformed commit/header/tree syntax without demanding missing ancestral history. Independent `cat-file -t` lookups and raw tree parsing check the actual commit-tree, included parent and tree-mode object types; this no longer trusts `ls-tree`'s mode-derived type. Exact snapshot/path closure and object counts/hashes/budgets remain unchanged. The original missing-identities and tree-as-blob reproducers now refuse before local actor-ref visibility.

The permanent hostile tests directly sign requests and send them through real temporary remotes even after local refusal. Their assertions require absent accepted/local actor refs, no visible events, no installed malformed target and no adopted authority. All 20 SHA-1/SHA-256 combinations passed independently, covering the original two findings and missing committer, malformed/duplicate author, bad date, NUL header, unsorted tree, noncanonical mode and included parent-as-tree. Additional permanent proof-hole tests prevent ambient object storage from filling missing mandatory pack members.

Valid snapshot-only behavior remains explicit: the positive real-remote test restores all three exact signed audit facts and the target snapshot while keeping the unreferenced target parent absent. I also ran that positive test independently under `GIT_DEFAULT_HASH=sha256`; it passed with a 1030-byte request pack, keyless historical reads and no implicit local anchor.

## Verification

- Original external reviewer overlay plus all `TestAudit*` tests: PASS, 22.367 seconds. `focused.log` includes both original reproducer outcomes and all permanent hostile submissions.
- Independent SHA-256 positive snapshot-only replication: PASS, 1.610 seconds, `sha256-positive.log`.
- Source head/tree confirmed unchanged after both checks; lane has only runtime `.spec-kitty/` metadata untracked. `git diff --check` passed. Correction touches only audit_protocol.go, audit_protocol_test.go, store.go and docs/audits-v1.md; no frozen release, module or tag surface changed.
- Reused full race suite at the exact source: PASS exit 0, root package 536.706 seconds; all three packages passed. Independently recomputed compressed digest `5fb751659be15fd1cc1af485ecd68adb4635989bd31911e09d2e5ceb808aea69` and decompressed raw JSON digest `48a56247cddae792896368870fc8bcf2c27e0c636e837257d4954c0bee4580ba`, matching `../corrective-cycle1-6d687af5/`. No duplicate full suite ran.
- Implementer vet/build/format/diff evidence was read at this fixed source. The completed canonical pre-review framework run reports no_coverage for missing Python tests.architectural, blocked=false and force_bypassed=false; this remains a disclosed infrastructure coverage gap, not a claimed covered framework gate or a Go failure.

## Requirement and anti-pattern checks

FR-001 PASS: old event omission/encodings and explicit disabled defaults. FR-016 PASS: distinct audit kinds and rejection as proposal/CI evidence. FR-018 PASS: strict signed schemas, exact provenance/relationships, bounded findings/logs and real replication. FR-019 PASS: explicit named private anchor, current branch ancestry, roles, stale/candidate/revoked refusal and historical readability. NFR-001 PASS: pack expansion/object/count budgets and per-selection charging. NFR-002 PASS: isolated structural/type/closure refusal and accepted-state preservation, including the repaired P1.

Anti-pattern checklist: dead code PASS (live store/quarantine/shallow validators; deliberate exported foundation API seam to WP03); synthetic-fixture test PASS (real production boundary assertions, graph-only unit fixtures are supplementary); silent empty return PASS (documented no-policy/non-audit/success helpers); FR coverage PASS; frozen surface PASS; locked decision PASS; shared-file ownership PASS (coordination below); production fragility PASS (bounded explicit validation errors, no new panic/exception path).

## Integration handoff

WP02/WP03 intentionally share lane-b and docs/audits-v1.md under lanes.json. WP03 supplies public audit command/execution/follow-up entry points on top of these exported foundation APIs; that scope is not represented as shipped by this foundation. WP03 should start at this corrected approved source. WP05 and WP12 must reuse the corrected audit validator for native snapshot admission, maintaining exact signed bytes and historical objects without adopting the private authority anchor or weakening existing native admission. Existing migration handoff/advice files remain applicable. No other WP status was edited by this reviewer.
