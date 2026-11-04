# Runner-efficiency domain model

| Entity | Identity / relationship | Meaning |
|---|---|---|
| Run request | Existing signed event ID; candidate, pipeline, definition, commit | Existing policy governs requester/execution. |
| Withdrawal | New signed event, Subject=request ID, Body=reason | Original request actor, later in its chain; future-start control. |
| Current results | Existing facts, latest per actor across authorized matching requests | Existing trusted threshold; no evidence reinterpretation. |
| Proposal lineage | Existing native index | Superseded/merged/closed/conflicting state determines usefulness. |
| Remote target observation | Selected exact branch ref + advertised OID | Bounded snapshot, no authority/fetch/ref rewrite/lock. |
| Scheduling decision | Eligible, skip(reason), refuse(error) | Derived view, never a synthetic result or permission. |

Withdrawals are one-way per request. Exact retries discover the original fact;
a new request represents new intent. Existing valid result/merge facts persist.
Default initialization/inspection create no queue/service/policy.
