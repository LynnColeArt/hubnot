# Work packages: opt-in policy rules

Three dependency-ordered packages on computed execution lanes. Planning and consolidation target: feat/opt-in-policy-rules. Main integration requires subsequent review.

## WP01: Protected check policy and enforcement

Dependencies: none

Prompt: tasks/WP01-protected-check-policy-and-enforcement.md

T001 Write failing protection/legacy policy tests, including forged acceptance and withdrawal. (WP01)
T002 Add optional protectedChecks and automerge rule types and strict bounded validation. (WP01)
T003 Implement bounded literal-path diff and shared designated approval evaluation. (WP01)
T004 Integrate status, acceptance evidence/admission, and current opted-in manual merge gates. (WP01)
T005 Expose optional policy sections in inspection/comparison; verify and commit. (WP01)

## WP02: Explicit native rule based automerge

Dependencies: WP01

Prompt: tasks/WP02-explicit-native-rule-based-automerge.md

T006 Write failing native rule, preview and execution tests. (WP02)
T007 Implement deterministic named-rule evaluation and read-only default preview. (WP02)
T008 Implement explicit execution using current native merge gates and expected target. (WP02)
T009 Verify stale evidence/target, author/path/actor mismatch and manual repair safety. (WP02)

## WP03: Public acceptance and optionality documentation

Dependencies: WP01, WP02

Prompt: tasks/WP03-public-acceptance-and-optionality-documentation.md

T010 Add black-box public CLI tests for opt-out, protections and explicit automerge. (WP03)
T011 Document opt-ins, configuration, exact evidence, limitations and manual recovery. (WP03)
T012 Run default/nogithub acceptance and all release checks, recording exact evidence. (WP03)
