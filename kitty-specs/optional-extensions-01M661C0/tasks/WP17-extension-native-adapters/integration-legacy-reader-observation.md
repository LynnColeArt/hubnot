# Inherited native malformed-head observation

Root scope record, 2026-10-31. WP17's temporary public fixture points one native
actor ref directly at an empty Git blob. The new host adapter now validates each
selected source head with the existing event decoder/actor binding and correctly
returns native/invalid_history. It must never turn that malformed history into an
empty successful host view or a verified operation absence.

The requested ordinary `hn issue list --json` control against that same fixture
exited 0 with ok=true and an empty item list. The actual output is retained in
external optional-extensions-WP17/cancellation-first.log under
TestExtensionNativeInvalidHistoryCLI. This was development-source execution,
not a test of a published release binary. The shared issue_read.go bytes are
identical between the approved beta foundation f8c68beb and WP17 WIP6be2eb29;
external wp17-legacy-reader-observation/source-proof.json records both hashes.

The adapter-owned verification closes the current host boundary. No shared-reader
or default native behavior change is silently included in this package. Carry the
inherited default-CLI malformed-head case into the separate beta compatibility/
recovery mission, with its own reproduction, ownership, regression and review.
Do not describe this observation as fixed in the ordinary CLI, close existing beta
issues on its strength, or infer stable-beta acceptance from extension checks.
Final extension readiness must retain this known separate follow-through.
