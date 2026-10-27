# WP05 independent correction review — approved

Source: `247a4e1b26295459ae159674c109bcd91aa1a954`, lane-d.
Reviewer: root Codex. Root did not edit product source in this review.
The landed patch was initially authored by a separate implementation child and
completed/validated/committed by Kat. This review inspected and tested that fixed
source independently; it does not claim another external reviewer completed it.

## Findings resolved

The complete change from bc6c8db5 modifies only migration_native_test.go (+17/-6).
Marker initialization precedes ApplyMigration. Refusal, observation-ref absence
and untouched marker assertions precede the control. The control uses existing
destination HEAD and requires both a Git failure and fresh DESTINATION marker.
The production resolver remains byte-identical to the previously reviewed P2 fix.

Independently ran `go test -race -count=1 -run '^TestMigration' .`: PASS (8.556s).
Three separately executed Go overlay checks exit1 at the intended assertions:
- Exact old production from 9ab00d85 (ENOENT fallthrough): fails with UNSELECTED
  hook lines in the marker.
- Explicit fixture hook activity: fails at the no-hook assertion.
- Missing-object control: fails the control assertion with empty marker evidence.

These are actual assertion failures, not compiler/environment errors. Overlays
operate only on disposable local fixtures and leave tracked source unchanged.
No additional blocker found in this correction; earlier production P1/P2 and
checkpoint/destination recovery corrections remain covered.

## Validation

All 19 checksum entries in Kat's supplied bundle verify. All 20 regular archive
members match the supplied bundle byte-for-byte. Both bound source hashes match
current files. Full-race JSON confirms 1,284 test/subtest pass events, zero fail
events, four passing packages; root output 534.928s, outer package event 587.776s.
All eleven supplied exit records name this source and report 0. Reuse this completed
full race/vet/build/format/variant/recovery evidence rather than duplicating the
full suite. Root additionally ran go vet ./..., gofmt on the changed test and
git diff --check bc6c8db5..HEAD successfully. Core production files are unchanged.

The pre-review framework gate completed with unavailable architectural authority
(no_coverage); this is neither a Go test failure nor a verified architectural pass.
No gate optout or force-bypass was used. Current noforge runs demonstrate tag
compatibility only; real adapter exclusion is the downstream WP06 requirement.

## Checklist

1. Dead code — PASS for this correction (no new production API). Broader WP05
   engine API is intentionally consumed by later provider/CLI WPs in the approved
   dependency plan; final integration must prove real callers.
2. Synthetic/ineffective assertions — PASS: actual engine/Git calls; all three
   negative probes now falsify the intended assertions.
3. Silent empty return — PASS: production unchanged from prior reviewed refusals.
4. FR coverage — PASS within WP scope: real local/archive/native/recovery tests
   and corrected no-hook regression; hosted adapters and CLI remain later WPs.
5. Frozen surface — PASS: only the owned test changes since bc6c8db5; historical
   snapshot/archive/release surfaces untouched.
6. Locked decision — PASS: no authority adoption, workflow execution, hook disabling,
   mutable snapshot refresh or blind uncertain-write retry added.
7. Shared ownership — PASS: no new shared file edit; existing project_snapshot.go
   maintenance retains its previously recorded WP04/WP05 ownership handoff.
8. Production fragility — PASS: no production code change, panic or new retry.

## Record and integration obligations

The old unrecorded 9ab review remains preserved as non-authoritative context;
do not manufacture a historical event or count administrative lease releases as
additional implementation rejections. Current approval is tied only to 247a4e1b.

The approved plan intentionally sequences engine → optional providers → public
CLI. Treat live reachability as a required WP06/WP11 integration obligation,
verified again by WP12 mission acceptance. This is WP-level dependency approval,
not release acceptance or a claim the migration commands are already wired.
Unused helper APIs must be connected to real consumers or removed at integration;
no dummy callers. Do not claim an adapter-free build before WP06 proves it.

Verdict: **approved for WP05**. Public main, tags and releases remain unchanged.
