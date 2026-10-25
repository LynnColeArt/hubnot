# Implementation plan: opt-in protected checks and automerge

## Summary

Extend the existing base-bound policy with two optional typed sections. Reuse native review, decision and merged events; no new event kinds and no dependency on provider adapters. Deliver on feat/opt-in-policy-rules for public review before any later main merge.

## Technical Context

**Language/Version**: Go 1.26 (existing go.mod).
**Primary Dependencies**: Standard library and ordinary Git; no new Go modules.
**Storage**: Existing signed Git actor histories, proposal refs and versioned .hn/policy.json.
**Testing**: go test -race ./..., go vet ./..., go build ./..., nogithub build/test, public native CLI fixtures.
**Target Platform**: Existing native CLI platforms.
**Project Type**: Single Go command with optional provider compilation.
**Constraints**: Policy 1 MiB existing bound; at most 64 paths/reviewers/rules/actors/authors; paths 4096 bytes; diff stdout 8 MiB and bounded stderr. No unbounded path capture.

## Charter check

HN-001 native Git only; HN-002 exact evidence; HN-003 bounded untrusted changes and explicit automation; HN-004 revisions need fresh evidence; HN-005 docs/protocol/threat changes accompany code. Tests precede implementation. Existing policy bytes and missing optional sections keep legacy behavior.

## Decisions and architecture

`protectedChecks` optionally contains `paths`, `trustedReviewers`, and `requiredApprovals`. Literal paths identify files or directories ending in `/`; reject empty, absolute, noncanonical, control-character, backslash, traversal and wildcard paths. Opting in automatically protects `.hn/policy.json` and each required `.hn/pipelines/<name>.json`. Use base/head tree differences with rename detection off and NUL-delimited names, so deletion/rename/type/mode changes are visible. A changed ancestor of a directory is protected as well. No semantic lint weakening detector.

Qualifying protection reviews are current, exact-candidate, distinct designated actors other than the proposal author. They may also count as ordinary approvals if separately trusted there. Add their IDs to acceptance evidence. Use one shared evaluator in status and current readiness; use the same rules in immutable acceptance validation/admission. Opted-in manual merge must recheck current readiness and protection even when a past acceptance exists. Legacy absence preserves existing manual semantics. A prospective manual merge gate loads the current target policy: if protection is enabled there and its digest differs from the candidate base digest, require a revision before merging. This closes newly authored old-base candidates without retroactively invalidating historical decisions. Both local validateDecisionEvent and separate quarantine validateReplicationDecision/merge admission must share the base-bound protection evidence validator.

`automerge` optionally contains bounded named rules with `targetBranch`, `actors`, `authors`, `paths`. Rule actors must be existing maintainers; author lists and paths are explicit and nonempty. Rule paths are literal, never executable expressions. `hn automerge PROPOSAL --rule NAME` previews; `--execute` opts into local merge. Default preview needs no signing key and writes no event/ref/index state. Print deterministic eligibility/blockers with exact target and evidence. Missing configuration cannot authorize execution.

Automerge uses existing review/CI/acceptance requirements and refuses current trusted change requests/rejections, policy amendments and protected changes. No auto-accept or fabricated approval. Require current checked-out target equals signed base, matching exact base policy and signed code ref. Changed paths all match the allowlist and the author and selected executing maintainer are explicitly authorized. No code/pipeline execution, fetch, sync or push.

Share existing merge and recovery semantics through a narrow entry seam rather than copying cmdMerge. Re-evaluate policy, evidence, rule, branch and expected target immediately before branch advancement. Preserve expected-ref compare-and-swap on advance; refuse target movement and concurrent evidence changes where observable. Do not rely on a preview result as execution authority. Expose existing missing-event repair as manual hn merge; retrying automation must not manufacture a second merge. Use a local serialization/expected-head design appropriate to existing Git worktree behavior; do not claim an atomic distributed transaction against arbitrary external writers.

## Control flow

```text
signed base policy + signed head + verified events
                    |
                    v
       native readiness + optional protected reviews
                    |
         current maintainer acceptances
                    |
       optional named rule + explicit execute
                    |
     final state/evidence/target recheck -> native merge fact
```

## Implementation concern map and work packages

- IC-01 / WP01: optional policy model, bounded path rules, shared protected evaluation, status/decision/admission/manual-merge gates, policy display/amendment comparison, focused regression tests.
- IC-02 / WP02: native automerge command and rule matching, preview, expected-target execution and recovery, focused tests. Depends on WP01.
- IC-03 / WP03: public CLI default/nogithub acceptance, docs (governance/protocol/threat/README) and full release verification. Depends on WP01 and WP02.

## Risks and mitigations

Path protection does not cover unlisted indirect scripts or arbitrary source-level lint suppression: document selecting all relevant files and keep review authority explicit. An actor with direct raw Git/host authority can bypass this app; do not promise host access control. Old Hubnot versions reject unknown optional fields, intentionally failing closed for enabled repositories; unconfigured repositories remain compatible. Default preview must not call signing-key migration. Target movement requires a revision, avoiding untested integration trees. Document migration without turning either feature on in this repository's own policy.

## Planning review resolutions

Independent review found the pre-opt-in-base bypass and separate quarantine admission. Both are explicitly covered in FR-004/WP01. For automatic advancement, the expected base must reach the actual Git ref transaction, not only a check before launching git merge. A prepared update-ref transaction can lock the expected target while a safe index/worktree update completes; because auto requires target=base and head descended from base, the head tree can form the merge tree. Implementation must verify this or use an equally sound lease, preserve unrelated work on refusal, and retain signed merge fact/repair semantics.

## Read-only automatic preflight

Existing Git readers can lazily fetch promised objects, and shallow-dependency classification may write recovery metadata. Avoid a broad Git/shallow API refactor: auto preflight reads bounded local metadata and refuses shallow or configured partial/promisor repositories before object reads. This is an explicit first-version auto-only support boundary; existing manual commands retain their behavior. Public acceptance proves full default/nogithub and keyless preview, plus metadata-only refusals.
