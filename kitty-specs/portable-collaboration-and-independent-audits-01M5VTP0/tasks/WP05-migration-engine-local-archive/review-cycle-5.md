---
affected_files: []
cycle_number: 5
mission_slug: portable-collaboration-and-independent-audits-01M5VTP0
reproduction_command:
reviewed_at: '2026-10-26T22:33:52Z'
reviewer_agent: codex-root
wp_id: WP05
---

# WP05 independent review of bc6c8db5 — test correction requested

Reviewed source: `bc6c8db5f6292d53509c068b11302e91a80a3ce0`, clean lane-d.
Reviewer: root Codex; no product edits. Kat's production P2 correction is verified.

## P2 — Preserve hook evidence and prove the control reached its hook

In `migration_native_test.go:778–784`, the marker is created/truncated AFTER
ApplyMigration. Every subsequent empty-marker/no-UNSELECTED assertion examines
bytes written by the test itself, so it cannot detect a hook that ran during
migration. The comment says “pre-create,” but the ordering does the opposite.

A review-only fault-injection overlay deliberately runs the fixture's UNSELECTED
hook and checks that its marker exists immediately before the landed reset.
The unmodified remaining assertions still PASS: `test-injected.log` records
UNSELECTED twice followed by a passing regression. The injected hook is a test
fault, not a new observed defect in Kat's product correction.

The control at line 796 has a related false-positive path: the source snapshot
OID has not been copied into the destination because the corrected resolver
refuses before object copy. Its update-ref errors without executing any hook.
Independent probe output confirms empty control markers. Using an OID already
present in the destination (for example its setup HEAD) executes the rejecting
DESTINATION hook, as shown by the stronger control's marker output.

**Required correction, test only:**
- Create the marker before ApplyMigration; never truncate it after the effect
  being observed. Read/assert its emptiness and observation-ref absence before
  running any control.
- Use a known existing destination object for the anchored control (or explicitly
  stage its required object), then assert both failure and a fresh DESTINATION
  marker. Do not accept an arbitrary Git error as proof of hook execution.
- Retain the actual migration call, meaningful refusal/ref-absence assertions
  and healthy bare/ordinary/linked regressions. No production refactor is needed.

The old regression does detect the original unrestricted successful promotion via
applyErr/ref checks. The finding concerns its separate promised no-hook assertion
and its claimed positive control, which are currently ineffective.

## Production correction and verification

The missing reciprocal pointer is now refused before promotion. A strengthened
independent production-boundary probe PASSes: migration returns an uncertain-write
error caused by invalid anchor, observation ref remains absent, and no hook runs.
An existing-object selected-destination control then demonstrably runs and rejects.
Healthy layout cases and all migration cases PASS in the same focused race run
(8.736s). go vet ./..., go build ./..., gofmt for both changed Go files and
git diff --check 13922bea..HEAD independently PASS.

Verified all 18 inner bundle checksum entries, every one of the 19 archive member
bytes against the supplied bundle, and all five bound current source hashes.
Archived full-race JSON has 1,284 passing test/subtest events, zero fail events,
four passing packages, root output 532.604s (outer package event 585.606s).
The existing exact-head full run is reused; no duplicate full race run.

## Required checklist

1. Dead code — PASS for corrected production surface: resolver is called by
   migrationPromote; this correction adds no public production API. Existing
   engine consumers assigned to WP06/WP11 remain integration obligations.
2. Synthetic/ineffective assertion — FAIL: hook-marker observations are erased
   before assertions; the control error alone does not prove hook execution.
3. Silent empty return — PASS: new refusal propagates an explicit anchor error.
4. FR coverage — FAIL for the required no-hook regression evidence above;
   independent stronger probe verifies the production fix itself.
5. Frozen surface — PASS: correction changes only native bridge and its tests.
6. Locked decision — PASS for examined production correction: expected-old
   transaction, isolated configuration and selected healthy hooks retained.
7. Shared-file ownership — PASS: two WP05-owned files; no new ownership conflict.
8. Production fragility — PASS: missing/unreadable/bad pointer refuses loudly;
   no new panic or uncertain-write retry introduced.

## Review record reconciliation

The old unrecorded review-feedback-3.md is retained unchanged as context for
9ab00d85. It is not a historical event-log verdict and is not backfilled as one.
The recorded direct P2 review and administrative release govern the latest fix.
Administrative ownership releases are not additional actual implementation reviews.
No completed review or model identity is inferred from a failed dispatch.

WP05's engine API is deliberately delivered before provider/CLI consumers;
WP06/WP11 wiring and WP12 mission acceptance must prove actual live CLI reachability
and real adapter exclusion. This correction does not settle those integration
obligations or claim current tag compatibility proves an adapter-free build.
No dummy call sites, unrelated helper removal or out-of-scope files are requested.

Verdict: changes requested, limited to the concrete regression-test issue.
WP06/WP11/WP12 wait for corrected-source approval. No dependent agents are active
in this reviewer's tree; this feedback is the available author/dependency handoff.
