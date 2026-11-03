# WP08 review feedback — cycle 1

**Verdict: changes requested.** Reviewer: independent Codex reviewer using
`reviewer-renata`, role `reviewer`. Reviewed report commit
`24a300538dc55011ccc30e3bce586d9b047b1697`, report SHA-256
`34e37b4c8b4e15bccebbcb0c46a2226f5323ad3a1395bf29dc1829a5e11bbf4b`.
The report author and this formal reviewer are different agents.

## P2 — Record unresolved Intel intake timeout without assigning a cause

**Location:** `docs/beta-boundary-review.md:128–135`, particularly the phrase
“failed H0/H2 fixture runs” at line 132. Requirement: T025/T026, FR-012 and
truthful qualification under HN-002. The surrounding package-pending language
is correct, but the collective fixture framing is no longer an adequate account
of the actual H0 evidence now available to formal review.

H0 run `40922372405`, job `123619663278`, source
`0b556b360c7ae5693b0d6e7cda875cece6b070a0`, executed the actual macOS Intel
standard and native-only package binaries. In both variants, the successful
contributor/maintainer/keyless-observer journey failed at
`beta_contribution_acceptance_test.go:547`: actual `hn contribution receive`
with operation `proposal-intake` was killed with
`deadline=context deadline exceeded`. The test helper sets a 45-second child
context. The same log records the other WP13 groups passing. This is distinct
from the later overall 30-minute Go test alarm, including the ARM job's overall
alarm. Neither a fixture defect nor a production defect is established as the
cause of the Intel child timeout.

**Evidence:**

- `work/beta-readiness-WP06/remote-artifacts/native-evidence-macos-15-intel-0b556b36/journeys.log:83`
  and `:127` record the two actual receive failures; lines 155–183 distinguish
  failed happy journeys from passing abuse/moderation/recovery groups.
- `work/beta-readiness-WP06/remote-artifacts/job-123619663278.log` and
  `work/beta-readiness-WP06/attempt-H0-mac-failures.json` retain job/outcome context.
- `beta_contribution_acceptance_test.go:72–93` establishes the 45-second harness
  context; line 547 is the actual proposal receive in the journey.

The `work/` evidence prefix above is
`/home/lynn/Documents/Codex/2026-10-24/wa/work/`. This observation reached the
reviewer after the author's report freeze; this feedback does not allege that
the author knowingly omitted it.

**Required remediation:** change only the owned review report to distinguish
known fixture failures from this unresolved Intel package-journey observation.
Record the exact run/job and affected operation/variants, state that its cause
is unproven and keep WP06 package qualification pending. Preserve the distinction
between supported source boundary conclusions and package/platform proof. H3
or a source full run remains active/pending until its own completed, bound
evidence is available. Do not infer a production bug, broaden a timeout/budget,
claim a package pass, or require a new runtime probe to complete this report
correction. No additional production repair WP is requested by this finding.

**Verification:** the revised report accurately names the observed child
timeout, separates the later aggregate alarm, removes the blanket fixture-cause
framing, and preserves exact source/evidence bindings and package-pending status.
Document consistency/whitespace checks suffice for this documentation-only
correction; WP06 retains the local full-test slot.

## Other review results and downstream impact

No additional source boundary defect was established by this formal review.
Private-state, bounded import/intake, aggregate accounting, direct-write,
authority, moderation/history, recovery and unsafe-runner conclusions were
checked against their actual source/assertion paths. All 217 current Go/module
hashes match the report's audited source; all 3,204 inherited closed evidence
entries and all 21 WP08 entries match. The framework handoff was completed
`no_coverage`, not an architectural test pass. No new Go/build/test/probe ran.

The mission has no `contracts/` artifact; WP08 introduces no concrete executable
contract or closed runtime value set. Anti-pattern checks: dead code N/A,
synthetic-fixture tests N/A, silent empty return N/A, new executable FR assertion
requirement N/A for this report-only delta, frozen surface PASS, locked decision
PASS, shared-file ownership PASS (exclusive report plus canonical lifecycle
metadata), production fragility N/A. Requirement truthfulness fails only at the
qualification wording identified above.

WP09 depends on WP08. Root should integrate/rebase onto the corrected,
independently approved report before relying on it for the final beta verdict;
no sibling status or source should change as part of this review.
