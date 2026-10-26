# WP03 source-bound implementation evidence

Mission: portable-collaboration-and-independent-audits-01M5VTP0
Work package: WP03 / T007–T009
Product branch: kitty/mission-portable-collaboration-and-independent-audits-01M5VTP0-lane-b
Approved dependency: WP02 6d687af50562e8d837915d39a97d82727e91fd71
Final source: 20923906ba24c939309540626d525a75729fbabe

## Behavior and boundaries

Seven owned product files add explicit native audit authority/rule/request/execute/read/follow-up commands. The registered entrypoint reuses approved WP02 authority, authorization/revocation, request, execution-validation and result-recording APIs. No policy roles or historical signed schemas are reimplemented. Existing runner isolation, exact target extraction, separate authorized definition bytes, current-policy rechecks and native issue writes remain the production boundaries. No automatic execution, scheduling, network fetch, issue publication, authority adoption or merge evidence is introduced.

Findings use capped inert structured JSON records with normalized stable keys; unstructured audit failure receives one stable rule-level finding. Runner transport failure and invalid output refuse result publication. Explicit follow-ups retain authenticated opening links and dedup by scope/rule/key across revisions, preserve operator edits, bind durable original selections, and use native actor-scoped issue operations to recover uncertain creation. Concurrent independent publishers remain visible and require explicit reconciliation.

Request operations use a hooks-disabled prepared Git verify transaction for local serialization, durable exact intent, and verified signed-fact recovery. No private guard ref is committed. Client termination releases the lock through pipe EOF; ordinary Git lock recovery remains necessary after hard group/power termination. This is a CLI orchestration guarantee, not distributed exactly-once behavior or a new low-level API guarantee.

Destination/operator reference-transaction hooks retain existing product behavior on actual signed/ref writes. The private uncommitted guard suppresses hooks. Source pipelines/hook files and foreign definitions stay inert during request, reads and import. This matches PRIMARY audit contract clarification f4c8329b and does not change app defaults.

## Public and responsibility-level verification

- Compiled ordinary and nogithub binaries: explicit anchor refusal/adoption, sole-owner authorization, exact separate source and old target snapshots, safe code-only authority advance, host opt-in, signed failing structured results, refused audit-as-run inspection, no automatic issues, linked follow-up retry/dedup, revocation refusal, real Git sync and keyless clone inspection without private authority.
- Compiled CLI candidate/stale-policy and owner/executor withdrawal refusals, missing authorization, invalid source/workflow selection, expected-actor mismatch, historical reads after withdrawal, and preserved accepted refs.
- Compiled CLI unstructured failure, pass, invalid structured output and capped 1 MiB log behavior; responsibility-level malformed/duplicate/overflow finding cases and transport/mid-run authority withdrawal refusal.
- Actual interrupted native follow-up issue creation followed by exact-selection retry; no issue flood, no overwrite of later operator issue edits, and durable conflict for reused operation keys including existing-only dedup.
- Concurrent compiled request invocations, explicit busy conflict, discarded acknowledgement recovery, changed-intent refusal, actual guard-client process termination/EOF recovery, and absence of committed guard refs.
- Public operator hook marker/refusal, inert source pipeline/hook files during request/read/real Git import, hook rejection preserving accepted refs, and retry after operator refusal.
- Public invalid UTF-8 argument regression: first reproduced an unintended signed request on prior source, then rejected invalid operation/source-path bytes before signed facts or private request intent writes on final source. Red/green logs are explicitly labeled.

## Validation provenance

The dedicated detached setsid supervisor records PID, start/end, exact before/after source and atomic exit JSON in /tmp. It runs go test -race ./... with the default timeout unchanged, go vet ./..., go build ./..., nogithub build/help, gofmt and git diff --check. Source file SHA-256 values and the exact owned product diff accompany raw logs. No live validation artifact is written to the product or primary task directory.

Prior source 3d116949 passed a full race run (main package 513.065s) and all other checks; this is prior-source evidence only and does not claim coverage of the UTF-8 correction. The final-source supervisor completed successfully: source before/after `20923906ba24c939309540626d525a75729fbabe`, PID/group 2420464, start 2026-10-26T12:33:44Z, end 2026-10-26T12:43:22Z. `go test -race ./...` passed (main package 524.365s; default timeout unchanged), and vet/build/nogithub build+help/format/diff checks all returned 0. The lane was clean and all seven source-file hashes reverified after completion.

Runtime qualification is Linux/amd64 with Go 1.26.4 and Git 2.43.0. Ordinary and nogithub public audit flows execute locally with explicit unsafe-host permission; this does not claim every platform or sandbox runtime. Adapter-free/noforge is not defined in this approved dependency lane, so an unknown Go tag is not presented as adapter-free evidence. Framework regression coverage/no_coverage, if reported by the later canonical pre-review transition, is separate from actual Go results.

## Evidence package

- Compressed raw archive: `raw.tar.gz` (21238 bytes).
- SHA-256: `ca48a75c7df891984782028da7606b3c535f8b970dc99107732dd918ec035ae9`.
- Archive includes final atomic exit, supervisor, source manifest and verification, exact owned diff, all final check logs, and explicitly labeled prior-head/red regression evidence. Generated binaries are excluded.
- Required canonical lifecycle handoff: copy this summary plus archive/digest into the PRIMARY WP03 task evidence directory and commit before marking T007–T009 done and moving WP03 to for_review. The pre-review framework gate must then be polled to completion without skips/force.
