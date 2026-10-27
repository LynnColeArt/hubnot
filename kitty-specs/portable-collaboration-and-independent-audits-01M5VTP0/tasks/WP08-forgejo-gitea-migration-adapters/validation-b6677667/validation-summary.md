# WP08 fixed-source implementation validation

Source: `b6677667c666148fc6cbe47d29d02a52b8bf0b5d`
Approved base: `dac3c4abbd4b5aaf40a5c31dd5395d64b8ef1116`
Lane: `portable-collaboration-and-independent-audits-01M5VTP0-lane-g`
Resolved implementation identity: codex / implementer-ivan. Existing canonical claim reused, never reclaimed.

All twelve changed source paths are WP08 owned. Source hashes were verified before the retry, after remaining checks, and after the completed review transition. Lane source is clean; no shared schema/registry/module/other-provider file changed.

## Completed Go checks

- Final focused: `go test -count=1 -race ./internal/forgejoapi ./internal/giteaapi . -run 'Test(Forgejo|Gitea|V1Migration)'` passed; root 9.439s. Tests call actual HTTP transport, registered adapters, saved migration engine, real Git composition and public legacy transfer commands.
- Original required `go test -race ./...` reached the 600s package timeout: root 600.200s, with `TestMemoryIndexTenThousandRecordsPerformanceAndRecovery` running for 1m36s. Runnable stack is JSON index encoding (`memory_index.go:518`, `memory_index.go:302`, `memory_index_test.go:1127`), outside WP08. Preserve `race.log`, `race-timeout-stack.txt`, hash and status as a real failed attempt.
- Root explicitly authorized one disclosed same-head finite-timeout retry: `go test -race -timeout 20m ./...`. It completed exit 0, root 644.228s, all packages passed. No source, test assertion, repository timeout/configuration, skip or filter change occurred. This passed full run supersedes the resource-limited attempt without erasing it.
- `go vet ./...`, `go build ./...`, formatting and `git diff --check` passed.
- Focused full-package tagged runs passed: `nogithub` root 40.405s retains and executes Forgejo/Gitea client/adapter/legacy tests; `noforge` root 28.265s exercises migration/native/archive/audit core with clients excluded.
- Ordinary, `nogithub`, `noforge` binaries built and each created/listed native issues plus the `hn gh issue list` facade with Git-only PATH, empty credentials and isolated HOME. Native private fixture HOME/repositories and binaries are excluded from the evidence archive.
- Ordinary and `nogithub` provider capabilities work without network. `noforge` rejects both provider registrations. Compiled file lists, dependency graphs and binary symbols independently prove actual adapter/client exclusion. No external module dependency was introduced. Git tags remained unchanged.

`remaining-checks.sh` was the continuation for previously unattempted non-full checks after the first timeout. Its inherited generic PASS line covers that subset only; explicit scope notes preserve this distinction. The independently logged `race-20m` exit 0 is the successful full-suite result.

## Lifecycle

- Canonical `mark-status T022 T023 T024 --status done` completed exit 0.
- Normal `move-task WP08 --to for_review` completed exit 0 and was polled to completion; no force or skip option/environment was used. WP08 is for_review, not approved.
- The framework pre-review gate returned `no_coverage`: authority `tests.architectural._gate_coverage` is not importable (`No module named tests.architectural`). This infrastructure authority remains unverified; it is not represented as a Go test pass. Actual source-bound Go checks above passed.
- Hosted sync authentication/consent warnings did not block the local canonical commands; no auth or sync consent change was made. Unrelated dirty/artifact warnings were ignored, not investigated or modified.

## Review focus

Consult interface-notes.md for qualified versions, real callers, conservative capacities, explicit mappings, unavailable board APIs, raw PR/review/asset/workflow preservation, and marker/receipt recovery rules. Both profiles have substantive issue/comment read/create/update coverage. Only fetched bytes are preserved; source users/reviews/workflows grant no authority or execution.

Independent review and downstream public `hn migrate` integration remain separate work. No self-approval, other WP lifecycle edits, live provider account writes, source deletion, pushes, public merges/tags/releases or keep-awake process changes occurred. Azure's separately coordinated project-only shared-plan fix is integration follow-through, outside this repository-bound WP08 source/gate.
