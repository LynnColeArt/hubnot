# WP02 corrective cycle 1 verification

Corrected product source: `6d687af50562e8d837915d39a97d82727e91fd71`.
Exact source tree: `c38c1bf8d5b2cd3534bd1c44fde2832e68c38409`.
Reviewed/rejected predecessor: `d274d5c8298f1e3beff3999ed6e2ee7d741a9c05`.

The two independent findings were reproduced before correction: `AuditRequest`
accepted both the missing-author/committer commit and the regular-file edge to a
tree, changing the signing actor ref. The original independent failing remote
reproducer remains in `../review-independent-d274d5c8/` and `review-feedback-1.md`.

The correction uses isolated `index-pack --fsck-objects` for Git syntax validation
and independently checks raw tree encoding/modes and actual object types for
commit-tree, included parent and tree-mode edges. Local appends now share that
isolated admission path with imported reads. Existing exact closure, role,
relationship, identity authority and object budgets remain enforced. Parent
history, unrelated policy/source proof-tree members and gitlink content remain
intentionally omitted. No public API, default opt-in, release/tag or dependency
changes were made. Changed product paths are audit_protocol.go,
audit_protocol_test.go, store.go and docs/audits-v1.md.

Validation at the corrected product tree:

- All `TestAudit*` cases passed (22.589s focused development run). New permanent
  local plus independently signed real-remote regressions cover missing identities,
  missing committer, malformed/duplicate author, invalid date, NUL header,
  unsorted tree, noncanonical mode, tree-as-blob and parent-as-tree, under both
  SHA-1 and SHA-256. Refusal preserves signing/accepted refs, event visibility,
  accepted object storage and private authority. Missing mandatory proof members
  refuse despite ambient object availability.
- Existing real-remote keyless/no-anchor exchange passes; it restores three exact
  signed audit facts and the target snapshot while explicitly leaving its
  unreferenced parent absent. The request pack remains approximately 0.9 KiB.
  Existing revocation/current-authority and audit-not-CI-evidence cases pass.
- Original independent reviewer overlay passes at exact corrected head (0.716s);
  see reviewer-reproducer.log and reviewer-reproducer.json. The permanent tests
  exercise remote admission even after local refusal, independently of that
  overlay's local-refusal short circuit.
- `go test -race -json ./...` passed exit 0. Source and final heads are identical.
  Supervisor PID 1590606, test PID 1590616; 2026-10-26T11:42:22Z through
  2026-10-26T11:51:27Z. Root package elapsed 536.706s; all three packages passed.
  No duplicate same-head full race suite was started; Go's default test timeout
  was unchanged. See run.json, exit.json, package-results.jsonl and supervise.sh.
- `go vet ./...`, `go build ./...`, gofmt and `git diff --check` passed on this
  unchanged product tree immediately before its source commit.

`race.jsonl.gz` is the complete nondated-gzip compressed JSON output. The original
raw JSON SHA-256 and empty stderr digest are in raw.sha256; compressed digest is
in compressed.sha256. The raw log is intentionally kept outside tracked evidence
in /tmp/hn-wp02-cycle1-6d687af5-race.jsonl; decompressing the tracked gzip restores
its exact bytes. Toolchain versions are recorded separately. No unexecuted host
or platform runtime qualification is claimed.

The canonical pre-review gate is invoked separately after this evidence commit.
The repository's missing Python tests.architectural hook has historically yielded
no_coverage; that is framework infrastructure evidence and is not a Go failure.

Canonical closeout: `mark-status T004 T005 T006 --status done` updated all three
subtasks through the CLI. `move-task WP02 --to for_review` returned exit 0 and
applied transition event `01M5W0XG8DMZ859WVCNJ1VEK1X`. Its pre-review gate was
polled to completion and returned `no_coverage`: tests.architectural._gate_coverage
is unavailable (`No module named 'tests.architectural'`). It reported zero new
failures, zero pre-existing failures, `run_state: completed`, `blocked: false` and
`force_bypassed: false`. No skip, force or disable flag was used. This records an
unverified framework gate alongside the independently passing full Go race suite;
it is not represented as a covered gate. See mark-status.json and for-review.log.
