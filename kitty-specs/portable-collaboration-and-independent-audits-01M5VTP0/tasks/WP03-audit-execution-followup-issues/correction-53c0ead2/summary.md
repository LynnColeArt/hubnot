# WP03 correction cycle 1 — source-bound evidence

Mission: portable-collaboration-and-independent-audits-01M5VTP0
Work package: WP03 / T007–T009
Reviewed source: 20923906ba24c939309540626d525a75729fbabe
Corrected source: 53c0ead287307736e7db9a0e6c63a135a4a9d2a0
Approved WP02 dependency: 6d687af50562e8d837915d39a97d82727e91fd71
Review feedback/evidence: PRIMARY ff4b388de80d28c90bd07983f6bea6e81b4e76e6, review-feedback-1.md / review-cycle-1.md. All archived review-evidence digests were verified before correction.

## Addressed finding

P2: Valid signed historical results can contain raw-unique finding keys such as X/x or whitespace variants. Follow-up previously retained both entries while deriving one normalized identity, created issues/private intent before deterministic operation conflict, and returned duplicate association entries on retry.

Correction: validate every selected normalized identity in a complete preflight before operation binding or native issue writes. Any selected identity collision, including ambiguous --finding selections, now returns an actionable audit_refused error directing the operator to select an unambiguous finding or use a result with distinct normalized keys. No original signed finding bytes are changed. The frozen WP02 schemas, writers, policy/authority checks, existing prepared request guard and operator-hook scope remain unchanged.

Owned correction delta is exactly audit_followup.go, audit_followup_test.go and docs/audits-v1.md. The code change is the missing whole-selection preflight plus explicit ambiguity handling; no unrelated feature or refactor is included.

## Red → green public regression

The regression creates otherwise-valid raw-unique case and whitespace collision results using production AuditRequest and RecordAuditResult. Each includes another ordinary finding so validation cannot be deferred until issue creation. Facts are inspected through the compiled public CLI, published through real Git sync and admitted into a separate local clone with a native follow-up signer.

For local and imported histories, the public CLI is exercised with whole-result selection, normalized --finding, original raw --finding and repeated operation. Every refusal must leave accepted actor refs, native issue count, complete private audit/intent directory state and the signed result representation unchanged. Error output must identify ambiguous normalized identity. The accepted raw historical finding keys remain byte-preserved and inspectable.

The red run on the reviewed implementation reproduced the defect for both collision forms and both repositories: the first whole-result invocation changed issue count 0→2, accepted actor refs and private intent state, then a retry returned duplicate associations. The selected-key ambiguity also exposed the old generic not-found error. Red evidence is explicitly labeled prior-20923906-red.log.

After the correction, the new regression passed, alongside existing positive compiled ordinary/nogithub cross-result dedup/replication and normal interrupted follow-up recovery preserving operator issue edits. The focused green run completed in 25.482s. This retains the original result/finding snapshot and authenticated-link behavior rather than changing accepted historical schemas or normal dedup semantics.

## Required fixed-source validation

One detached setsid supervisor runs the mandatory full go test -race ./... with the default timeout unchanged, followed by go vet ./..., go build ./..., nogithub build/help, gofmt and git diff --check at the corrected source. Its PID/start/end, exact before/after source and exit results are recorded atomically. No unchanged prior-source full suite was duplicated.

Final-source validation completed successfully. PID/group 3285180; start 2026-10-26T16:04:27Z; end 2026-10-26T16:14:31Z. Source before and after is exactly 53c0ead287307736e7db9a0e6c63a135a4a9d2a0. Full go test -race ./... passed (main package 550.843s, default timeout unchanged), and vet/build/nogithub build+help/format/diff checks each returned 0. The lane remained clean; all seven owned source-file hashes were verified after completion. No old-head full suite was repeated.

Runtime qualification remains Linux/amd64 with Go 1.26.4 and Git 2.43.0. Ordinary/nogithub public flows and explicit host execution are exercised; no new sandbox or other-platform runtime claim is made. Adapter-free/noforge is unavailable in this approved dependency lane and is recorded separately. The later canonical pre-review framework result, including no_coverage for a missing tests.architectural module if applicable, is separate from actual Go check results.

## Handoff

Final archive/digest/summary must be copied and committed by root into the PRIMARY WP03 task evidence directory before canonical T007–T009 completion and the for_review transition. The mandatory framework gate will be polled to completion without skips or force. Live validation files remain only in /tmp; product lane contains only owned source commits.

## Final evidence package

Compressed raw archive: `raw.tar.gz` (6885 bytes). SHA-256: `3e44b3c8e009f67c022131583582bff49a4ad8c02b68ed246ee95832b70eb677`. The archive includes exact-source exit and supervisor records, all final check logs, source manifest/verification, three-file correction diff, review-digest verification and explicitly labeled red/green regression evidence. Generated binaries and private fixture data are excluded.
