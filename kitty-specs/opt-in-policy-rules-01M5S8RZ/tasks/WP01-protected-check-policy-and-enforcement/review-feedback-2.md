# WP01 post-approval adversarial finding: protected target baseline reversion

Reviewed source: `ae33a60ad9d74727ceee3207c575b84deec7af87`.
Reviewer: independent reviewer-renata / policy_plan_review.
Severity: blocking correctness/authority gap, FR-002/003/004.

## Reproduction

Protection is already enabled at signed base B under policy P. Target T descends from B and tightens `lint.conf`, without changing policy bytes P. Candidate H descends from T, restores `lint.conf` to B's weaker bytes, and modifies `docs.txt`. The new candidate signs base B and head H. All ancestry is valid, and target policy digest still equals the candidate base-policy digest.

A scratch Go overlay using existing native Git/event fixture helpers reproduced the issue without modifying product files:

`go test -overlay /home/lynn/Documents/Codex/2026-10-24/wa/work/wp01-baseline-revert-overlay.json -count=1 -run '^TestReviewProtectedTargetReversionSamePolicy$' -v .`

Observed output:

```text
signedDiff=["docs.txt"] actualTargetDiff=["docs.txt" "lint.conf"]
protected=[] approvals=0 ready=true
Recorded accept decision ...
Merged proposal ... into main ...
Recorded merge event ... with 1 acceptance decision(s)
merged lint.conf="strict=true\n"
REPRODUCED: zero designated approvals, target lint tightening reverted under identical policy bytes
```

The targeted test intentionally failed on the reproduced vulnerability in 0.300s. Actual target lint before merge included an additional strict setting; afterward it matched B's weaker version. Existing policy-digest stale-base protection does not detect this because policy bytes remained unchanged.

## Requested correction

Before a new manual merge governed by enabled protection, inspect protected-path changes between signed base B and the current target under the applicable protected policy. Require a new proposal revision against the current target if any protected path changed, in addition to the existing target-policy digest guard. An equivalent exact merge-tree protection design is acceptable if it preserves exact review evidence and blocks this case.

Do not invalidate historical signed decisions or change contained historical merge-event repair semantics. Preserve absent optional configuration and ordinary legacy behavior. Add a regression with B < T < H, identical policy bytes, target tightening followed by a hidden candidate reversion, and zero designated approvals; assert refusal before target advancement. Also verify stale-base proposals remain eligible when no protected target paths changed, unless another existing gate blocks them.

This finding supersedes the earlier WP01 approval for the reviewed source. No product source or lifecycle files were edited manually during review. WP02 depends on WP01 and must incorporate the reviewed correction before final verification.
