# WP03 correction-cycle independent review

Verdict: PASS, approval recommended. No remaining actionable finding was identified.

Source: `53c0ead287307736e7db9a0e6c63a135a4a9d2a0`.
Approved WP02 dependency: `6d687af50562e8d837915d39a97d82727e91fd71`.
Prior reviewed source: `20923906ba24c939309540626d525a75729fbabe`.
Prior rejection/evidence: PRIMARY `ff4b388de80d28c90bd07983f6bea6e81b4e76e6`, `review-feedback-1.md`.
Prompt: `/tmp/spec-kitty-review-prompts/nichthub-2bfda21ece60/portable-collaboration-and-independent-audits-01M5VTP0/WP03/a99826e885ac45b782931f584a045ac4.md` (read in full).
Contract: PRIMARY `contracts/audits-v1.md`, retaining hook clarification `f4c8329b`.

## Finding resolution

The three-file correction is limited to `audit_followup.go`, `audit_followup_test.go` and `docs/audits-v1.md`. Follow-up now derives and validates all selected normalized identities before operation-intent binding or native issue writes. Ambiguous explicit `--finding` selection reaches the same actionable refusal. The selected slice contains value copies; original signed findings and historical schemas remain unchanged. Selecting one unambiguous finding from a mixed result remains supported.

The producer-side structured-output rejection is no longer the sole protection. New compiled CLI regression fixtures sign accepted raw-unique collision results through production writers, replicate them over real Git, inspect local/imported histories, and prove that whole-result/repeated/raw-key/normalized-key refusals preserve accepted refs, issue count, private intent state and the signed result. An earlier valid finding proves complete preflight happens before any selected create. The regression's prior-source red evidence and corrected-source green evidence were inspected.

No authority/rule/schema/runner/request-guard changes were introduced. Existing source and target hooks/pipelines/foreign definitions remain inert during request/read/import. Actual operator-controlled destination reference-transaction hooks retain Git behavior; only the private prepared guard suppresses hooks. No global Git configuration override was made.

## Independent verification

- Implementation correction archive digest verified: `3e44b3c8e009f67c022131583582bff49a4ad8c02b68ed246ee95832b70eb677`.
- Archived correction diff matches the actual three-file diff byte-for-byte: SHA-256 `3d7b0a36a8a4b62f875a7e0a8f66588c16691975a61afef276698dd5acdbcf2d`.
- All seven owned current source hashes match the archive source manifest. Archived atomic exit record identifies exact `53c0ead2` source before/after, and full race log records `ok hubnot 550.843s`; race/vet/build/nogithub build+help/format/diff checks each exited 0. No unchanged full suite was duplicated.
- Independently reran focused existing checks with `go test -race`, exit 0 in `29.931s`: compiled ordinary/nogithub explicit audit/follow-up/real-Git replication; local/imported case/whitespace collision refusal; interrupted native create recovery preserving operator edits; real destination operator hook and inert sources; UTF-8 refusal; runner transport/mid-run authority withdrawal; prepared-lock client termination release.
- Additional independent overlay probes passed with `go test -race`, exit 0 in `10.844s`. A Kelvin-sign/ASCII-K collision with EM SPACE trimming is rejected across five whole/explicit/reused/default operation selections before accepted refs, private intent, signed result or issue count changes. Selecting the valid finding from the mixed result and processing a later normalized equivalent result both return the original issue and preserve its title. False request links do not suppress creation; multiple genuine associations remain visible; scope/rule identities remain distinct.
- Review probes were compiled only through a temporary Go overlay. No product files were edited. Final source stayed `53c0ead2`; the lane has only its pre-existing untracked `.spec-kitty/` review lock.

## Required anti-pattern checklist

1. Dead code: PASS — `localCommands["audit"]` reaches `cmdAudit`, command dispatch, execution and follow-up production code. The correction adds no public production API; the preflight is on the live follow-up path.
2. Synthetic-fixture test: PASS — behavior assertions invoke production signed-fact writers, compiled CLI, real Git sync/admission and native issue writes; the independent overlay follows those boundaries.
3. Silent empty return: PASS — the correction adds an explicit actionable error, not an empty success or swallowed failure. Existing empty findings/history states retain documented meaning.
4. FR coverage: PASS — FR-001 existing/default/required-evidence isolation; FR-017 explicit sandbox/host selection and no automatic execution; FR-018 verified provenance/current authority/logs/findings; FR-019 anchored roles/rules and withdrawal; FR-020 authenticated stable dedup and original-intent recovery, now pre-write collision refusal; FR-022 current docs; NFR-001 positive bounds; NFR-003 fixed-source checks and public acceptance are substantiated by current and unchanged source-bound evidence.
5. Frozen surface: PASS — total WP03 delta from approved WP02 is the original seven owned files; correction delta is exactly three. No changes to frozen WP02 implementation, historical signed bytes, event/store/quarantine/shallow admission, backend or pipeline runner.
6. Locked decision: PASS — complete selected-identity validation precedes mutation; explicit authority/execution, inert imported definitions, native merge-evidence separation and no implicit provider action remain intact.
7. Shared-file ownership: PASS — `docs/audits-v1.md` deliberately extends approved WP02 documentation in the sequential shared lane. The six other WP03 files remain exclusively owned; root coordinated the verdict mutation window.
8. Production fragility: PASS — the new deterministic ambiguity refusal is pre-write and actionable; no panic, transient-race exception or hidden partial success is introduced. Existing native actor-operation recovery is retained.

## Limits

Actual runtime evidence is Linux/amd64 with Go 1.26.4 and Git 2.43.0. Public host execution remains explicit; no additional sandbox or macOS/Windows runtime qualification is claimed. `noforge` is not yet defined in this approved dependency lane, so neither an unknown build tag nor `nogithub` is claimed as all-adapter exclusion. Framework `no_coverage` for missing `tests.architectural` is recorded separately from the passing actual Go checks and was not bypassed. Concurrent independent publishers can still create multiple visible associations; no distributed exactly-once guarantee is claimed.
