---
affected_files: []
cycle_number: 1
mission_slug: pinned-actor-operation-lookup-01M22QQS
reproduction_command:
reviewed_at: '2026-09-09T10:01:56Z'
reviewer_agent: codex-independent-automation-review
wp_id: WP01
---

# WP01 independent review — changes requested

Reviewer: `codex-independent-automation-review`, loaded `reviewer-renata`.
Source reviewed: `bfd9c5789573b205d80fd647b0059534cdcc3d76`.

## R1 [P2] Machine identity discovery silently mutates legacy private state

Location: `identity_continuity.go:379` (`cmdIdentityShow` calling `loadIdentity`); existing migration branch in `identity_keyring.go:467` (`loadActiveIdentity`).

The new `hn identity show --json` public discovery path uses the ordinary active-identity loader. When only a valid legacy `.git/hn/identity.json` exists, that loader creates `identities/<actor>.json` and `active` before returning the public response. The issue refs stay unchanged, so the current ref-only checks do not catch this. An automation client performing read-only binding inspection therefore repairs private provider state without requesting a mutation.

This is material to the intended prerequisite: Go Kitty's approved FR-001 and provider binding contract require inspection to be read-only and perform no init/repair. Reusing the existing loader is safe for signature identity but does not provide that behavior. The previously existing human loader/migration behavior need not change.

Independent public reproduction: in a disposable repository, write a valid legacy identity, inventory the private keyring paths, invoke the compiled `hn identity show --json`, then compare paths. It returns success and creates `active`, `identities`, and `identities/<actor>.json`.

Retained test-first evidence outside the repository:

- `/home/lynn/Documents/Codex/2026-09-09/h/work/hn-prerequisite-review/review_identity_test.go.txt`
- `/home/lynn/Documents/Codex/2026-09-09/h/work/hn-prerequisite-review/overlay.json`
- `/home/lynn/Documents/Codex/2026-09-09/h/work/hn-prerequisite-review/identity-readonly-red.log`

Command: `go test -overlay /home/lynn/Documents/Codex/2026-09-09/h/work/hn-prerequisite-review/overlay.json -run '^TestReviewPublicIdentityDiscoveryDoesNotMigrate$' -count=1 -v .` from the WP lane. Result: FAIL, 1.018 seconds, explicitly reports created private keyring paths; no private contents were printed.

Required fix: give new machine discovery a nonmutating identity-read path. It may safely expose validated legacy public fields without migration, or return a typed actionable refusal until explicit migration occurs. Preserve default human compatibility; do not silently initialize, migrate, rotate or repair while inspecting. Add a public regression comparing private files/bytes/modes as well as refs for normal, legacy and invalid discovery. Update the public contract/documentation to make read-only semantics explicit. Do not broaden this into unrelated identity redesign.

## Other reviewed behavior

No additional blocker found. Actor pin is checked against the captured identity before replay/append; signing still validates event/signer identity. Operation lookup uses one verified bounded reader, exact actor/key matching, explicit duplicate/absence/error outcomes and original signed fields. Detail pages expose unchanged signed opening metadata after current rewrites and during conflicts; legacy output is an empty object, not null. Summary defaults/cursor fingerprints remain compatible. Owned diff contains exactly seven intended files, no key, binary, dependency or frozen-history changes.

Independent `go test -race -run '^TestAutomation' -count=1 -timeout=90s -v .` passed in 3.445 seconds; raw log retained at `work/hn-prerequisite-review/focused-race.log`. Implementer complete race log passed in 248.573 seconds and was inspected; vet/build logs are clean. No redundant full race was run. Source remained equal to the reviewed commit during verification.

## Required checklist

| Item | Verdict | Evidence |
|---|---|---|
| Dead code | PASS | Identity and issue dispatch reach the new production helpers. |
| Synthetic-fixture test | PASS | Public subprocess tests assert actual commands; controlled signed fixtures set up conflicts, legacy and invalid histories. |
| Silent empty return | PASS | Empty legacy opening metadata is documented; lookup absence/errors are explicit. |
| FR coverage | FAIL | New public discovery is insufficient for the read-only consumer requirement; missing private-state invariant reproduced by R1. Other six-FR behaviors have executable coverage. |
| Frozen surface | PASS | Source commit modifies seven owned current source/test/document files only. |
| Locked decision | FAIL | R1 prevents the public prerequisite from satisfying the agreed no-repair inspection boundary; make that boundary explicit upstream. |
| Shared-file ownership | PASS | One WP owns the parser/identity/docs diff; opening_metadata was explicitly added to canonical scope before readiness. |
| Production fragility | PASS | Typed errors preserve expected validation/race/read failures; no new panic/fallback path. |

Verdict: **REJECT / changes requested for R1 only**. Reviewer made no implementation edits and did not merge or publish.
