# Independent planning review

Date: 2026-10-26. Reviewer: independent `portable_plan_review` context with
Reviewer Renata resolved through the canonical profile CLI. Review was read-only;
the reviewer authored no product code, Git changes or lifecycle state.

## Findings against initial plan d37847bf

1. **P1 — Current audit authority lacked an independent anchor.** Arbitrary
   `--policy-ref` could select a policy predating owner withdrawal or containing
   a candidate's self-appointed owners. Parsing that policy proves neither its
   current selection nor freshness. Define an explicitly adopted local authority
   anchor, constrain policy assertions to it, and separate historical validity
   from current execution authority. Tests must cover stale assertions,
   candidate owner lists and imported state that cannot change the anchor.
2. **P1 — Destination inputs omitted blobs and predecessor receipts.** An intent
   alone cannot create a comment on a newly created issue whose provider ID was
   assigned by the destination. Local/archive writes also need verified bundle
   and attachment bytes. Supply explicit verified blob access and confirmed
   dependency receipts to apply/reconciliation; resolve typed symbolic references
   through the engine and persist the resolved payload before its side effect.
   Freeze closed shared issue/comment schemas before provider parallelism.

Root accepted both findings and requested contract amendments before task
finalization. No gate was skipped or implementation approved on the rejected
contract.

## Correction verification

The same independent reviewer rechecked the amended plan, contracts, data model
and quickstart and approved canonical task finalization with both P1 findings
resolved. Audit authority now uses a privately and explicitly adopted named
branch anchor; policy assertions cannot replace it. Apply and reconciliation
receive verified blobs, confirmed predecessor receipts and the same persisted
canonical bound write. Closed shared issue/comment schemas are assigned to the
foundation packages before adapters begin. No remaining blocker was reported.

## Provider research review

Independent provider research approved the specification's honest capabilities
and preservation boundaries. It required non-vacuous per-adapter minimums:
issue/comment read/write for GitHub/GitLab/Forgejo/Gitea, mapped work items/comments
for Azure, and available repository/PR/workflow preservation for Bitbucket Cloud.
It also required supplied Azure bearer/PAT authentication and explicit supported
Services/Server API versions, without deprecated OAuth registration. These
requirements are incorporated in the spec and plan contracts.
