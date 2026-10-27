# WP05 implementation validation (test-evidence correction, cycle-5/6)

Source: `247a4e1b26295459ae159674c109bcd91aa1a954` (commit 247a4e1b, lane-d) on bc6c8db5 (P2 resolver fix).
Response to review-cycle-5.md / review-cycle-6.md — TEST-ONLY correction; the
production P2 fix was verified by the reviewer and is unchanged.

## What changed (migration_native_test.go only, +17/-6)

- The stale-registration regression now pre-creates the hook marker BEFORE
  ApplyMigration (the reviewed bug: marker was created after the effect, so
  empty-marker assertions proved nothing; the reviewer fault-injection and
  this rework's independent fault-injection both confirmed it). Refusal/
  ref-absence/empty-marker assertions all complete before the control runs.
- The anchored control now uses the destination's own setup-HEAD OID (already
  present in the destination object DB; a refused migration never copies the
  source snapshot object) and asserts BOTH the update-ref failure AND a fresh
  DESTINATION marker line, so the rejecting hook demonstrably executed.

## Independent fault-injection audit recorded by this rework

Reverting the production resolver to the exact cycle-4 ENOENT-fallthrough
shape (overlay, scratch only) makes the corrected regression FAIL at the
empty-marker assertion with UNSELECTED hook lines present — the test now
detects the defect it claims to detect. Healthy-layout regressions (bare/
ordinary/linked) remain green under the same corrected test.

## Checks (all exit 0 at this exact committed source)

- Full go test -json -race ./...: exit 0; root pkg 534.928s; 1284 Test+subtest
  pass events; zero failures; four packages pass.
- Focused go test -race -run ^TestMigration .: exit 0 (8.797s).
- go vet ./..., go build ./...: exit 0. gofmt: clean. git diff --check
  3269fabb..HEAD: exit 0.
- nogithub focused 29-test set + binary build: exit 0. noforge tag
  compatibility incl. internal/forge + internal/githubapi: exit 0.
- Recovery probes: exit 0. internal/migration dep inventory: no provider
  client packages. Missing-in-bundle log noted: none; all captured above.

## Scope and reconciliation

Test-only per cycle-5/6 required correction; no production files changed vs
bc6c8db5. Cycle-3 production-reachability owner decision remains OPEN and
outside this correction. Bundle SHA256SUMS covers every member.
