---
affected_files: []
cycle_number: 1
mission_slug: portable-collaboration-and-independent-audits-01M5VTP0
reproduction_command:
reviewed_at: '2026-10-26T11:33:17Z'
reviewer_agent: user
wp_id: WP02
---

# WP02 independent review — changes requested

Source reviewed: `d274d5c8298f1e3beff3999ed6e2ee7d741a9c05`.

## P1: Reject malformed snapshot object graphs before audit visibility

`audit_protocol.go:813–859` (`validateAuditObjectShapesAt`) is not a sufficient replacement for structural Git validation at the new non-strict `index-pack` boundary. Commit validation checks only a first `tree` line, a minimum line count and parent OID spelling; tree validation checks names and allowed mode strings but never checks each referenced object's actual type against its edge mode. `git ls-tree` reports the mode-implied type, so its printed type is not an independent object-type check. Recomputing an expected object set from the same malformed graph does not establish that graph's structural validity.

Independent tests through `AuditRequest` and a real temporary Git remote reproduced both cases:

1. A target commit whose content is `tree <valid-tree>\ngarbage header\n\nmessage\n` (missing author and committer) is accepted locally and fetched admission reports `promoted`; `collectEvents` returns both audit facts. Git independently reports `missingAuthor`.
2. A validly encoded target commit whose tree has a regular-file `100644 wrong-type` entry pointing to a **tree** object is also accepted locally and promoted remotely. Git independently reports “is a tree, not a blob” and broken links.

This violates the hostile/corrupt-object refusal guarantee in NFR-002 and the documented exact snapshot/type validation contract. No signature forgery, existing-history rewrite or extra undeclared pack object is required. The malformed objects travel inside signed `audit.pack` and enter accepted object storage/history.

Fix: validate structural commit syntax and tree edges against actual object types in the isolated pack database, with appropriate Git object-format rules, before installation or any accepted/local actor ref update. Preserve the deliberate snapshot-only design: missing commit-parent history is allowed; missing required target-tree/path-proof members, invalid object syntax and wrong edge types are not. Do not weaken the existing native admission checks or require complete ancestral history as a workaround. Add local and real-remote negative regressions for these cases, asserting no accepted actor ref/event visibility and no imported authority; retain positive snapshot-only replication coverage.

The complete reproducer, overlay configuration and concise failing log are in `review-independent-d274d5c8/`. Reproduce from the lane worktree with:

```sh
go test -overlay=/home/lynn/projects/nichthub/kitty-specs/portable-collaboration-and-independent-audits-01M5VTP0/tasks/WP02-native-audit-protocol-admission/review-independent-d274d5c8/overlay.json -run '^TestIndependentAudit' -v .
```

## Other review results and coordination

Existing `TestAudit*` focused cases passed independently (11.530 s). The previously completed full race suite passed at this exact source; its source/exit/log hashes are retained in the evidence summary. No duplicate full suite was run. Current authority ancestry uses the actually selected local branch; current vs historical rule projection, explicit opt-in, stale/revoked refusal, immutable prior event omission and observational non-merge evidence behavior otherwise inspected consistently with the contract.

WP03 deliberately owns CLI execution/follow-up integration and consumes these exported foundation APIs; their command entry points are a legitimate downstream integration seam. WP02 and WP03 share lane-b and `docs/audits-v1.md` by `lanes.json`; WP03 must start from the corrected approved WP02 source. WP05/WP12 and migration admission must likewise consume the corrected validator. No product changes or other WP lifecycle changes were made in this review.

Anti-pattern checklist: dead code **PASS** (production validator/store/quarantine/shallow paths and deliberate downstream command API seam); synthetic-fixture test **PASS** (real production boundaries, with graph-only conflict unit fixture supplementary); silent empty return **PASS** (nil helpers represent documented non-audit/no-policy cases); FR coverage **FAIL** (NFR-002 malformed object rejection uncovered); frozen surface **PASS**; locked decision **FAIL** (corrupt snapshot admission violates required refusal); shared-file ownership **PASS** (lane-b coordination above); production fragility **PASS** (explicit validation errors, no new panic/exception path).
