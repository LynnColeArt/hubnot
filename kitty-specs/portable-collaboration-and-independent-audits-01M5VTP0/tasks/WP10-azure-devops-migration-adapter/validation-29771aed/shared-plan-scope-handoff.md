# Shared plan-scope correction ownership handoff

Root coordinates this narrow WP05/WP10 ownership exception before editing.
Azure Work Items/Boards belong to a project and may have no Git repository.
The existing Locator already models Project and Repository separately, and
snapshot validation already permits an empty Repository. CanonicalPlan's
unconditional nonempty Repository requirement incorrectly prevents this scope
from reaching PrepareMigration, despite an otherwise explicit destination.

WP10's author may modify only the destination-scope check in
`internal/migration/plan.go` and add responsibility-boundary tests in
`internal/migration/plan_test.go`, alongside the normal WP10 files. Require a
nonempty Project OR Repository; keep all field bounds/URL/digest/identity checks
and reject completely unscoped (including organization-only) destinations.
Do not branch on provider names. Each actual provider still validates its own
scope; local/archive/GitHub/repository destinations retain their own requirements.

This changes no JSON field, schema/version, Source/Destination/ApplyContext,
receipt or write contract. It corrects the generic existing Locator semantics;
it is not a waiver of validation or testing. Test project-only success,
repository-only success, no-scope refusal and invalid selector refusal, plus a
real saved-session Boards-only Azure application with no repository sentinel.

Only the Azure author owns these two shared edits during parallel implementation.
GitLab/Forgejo/Gitea authors must leave them untouched. Their repository-bound
inputs continue to use the approved base. WP10 review must examine this exception
explicitly; final integrated mission tests must cover all provider combinations.

Target remains feat/portable-collaboration. No public main/tag/release action.
