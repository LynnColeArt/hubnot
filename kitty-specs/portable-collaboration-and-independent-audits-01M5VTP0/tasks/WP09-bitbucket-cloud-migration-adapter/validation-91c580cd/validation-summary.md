# WP09 implementation and validation handoff

Final implementation head: `91c580cda750a77493e933e086d0b535d4e7c724`.
Approved WP06 base: `dac3c4abbd4b5aaf40a5c31dd5395d64b8ef1116`.
Source commits: `5c7a67518b000505a10f6623f1156d52b4605937`, then
`91c580cda750a77493e933e086d0b535d4e7c724`.
Only the six WP09-owned implementation/test files changed. Lane source stayed
fixed throughout final checks and is clean. SHA256 binds every owned file.

The Bitbucket Cloud profile provides offline registration/capabilities and actual
bounded repository/PR/activity/comment/workflow/selected-attachment reads. Keys
scope immutable repository UUID by provider and normalized instance; workspace
and slug are selectors. Repository grouping projects never replace ProjectID.
Global PR roots/tombstones use inert reply records; code-anchored roots use
inline_comment; all replies preserve actual parent kinds and source coordinates.
Activity preserves raw provenance as content-identified observations, never native
review authority. Exact pinned pipeline bytes remain inert. Missing/private/deleted
content and removed issue APIs differ from a successful empty connection.

Frozen v1 cannot represent PR-parent writes. Destination Plan emits unsupported
findings and no API intents; Apply/Reconcile refuse without network or fabricated
receipts. Native/archive preservation and the shared Git companion are tested,
including native import with the provider already shut down. Public migration CLI
wiring belongs to WP11; production OpenMigrationSource/OpenMigrationDestination
registration seams and shared composition are exercised here.

## Actual final-head verification

- One final unfiltered `go test -race -timeout 20m ./...` passed, root package
  627.389 seconds. Started 2026-10-27T04:11:36Z; natural exit0 at04:23:07Z.
  It ran after the coordinator released the serialized full-suite slot.
- `go vet ./...`, `go build ./...`, formatting and working/committed diff checks
  passed. Ordinary, nogithub and noforge focused checks passed.
- Ordinary/nogithub/noforge binaries built. Compiled-file, dependency and symbol
  assertions prove nogithub retains Bitbucket while noforge strips its client
  and registration. Module inventory remains only hubnot.
- Each binary performed native init, issue creation/listing, local gh facade,
  transfer/GitHub help and version under env-i, isolated HOME and Git-only PATH,
  without provider credentials. These development fixtures remain outside the
  evidence bundle and have not been recursively deleted.
- Fixture tests cover opaque cursors, connection/origin changes, redirects, loops,
  page/record/wire/blob/manifest bounds, duplicate fields/IDs, UUID replacement,
  renamed selectors, actual empty/private/deleted content, orphan parents,
  exact workflow bytes, explicit Basic/Bearer credential profiles, token JSON
  exclusion, no unsupported writes and offline source composition/preservation.

The classification regression is independently reproducible: an isolated export
of old implementation5c with final91's regression test fails as expected (exit1,
three wrongly inline-classified roots vs the correct one). Six input hashes and
exact command/output are in classification-red-source/raw/command evidence.
Final91's normal focused and complete race checks pass the same regression test.
This expected old-code failure is not a final-head failing gate.

The earlier complete5c race PASS (763.298 seconds) and other5c gates are superseded
by91 and not reused as final-head proof. They remain at
`/tmp/hn-wp09-evidence.k9rRnH`; final raw evidence is separately stored under
`/tmp/hn-wp09-final-evidence.0VKmQg`.

## Canonical lifecycle and limitations

Canonical T025/T026/T027 mark-status completed exit0. Normal move-task WP09
--to for_review completed exit0; no force, skip or self-approval was used.
The framework architectural authority returned `no_coverage`/unverified because
`tests.architectural._gate_coverage` is unavailable. This is not claimed passed.
Hosted sync nonconsent/auth warnings and binary-artifact UTF-8 scanner warnings
are retained verbatim; no hosted sync was enabled or consent invented.

Go1.26.4's nested-worktree VCS stamp discovers the enclosing PRIMARY checkout
and embeds its revisionddce2120+dirty despite compiling lane-h package paths.
Verbose build trace proves this discrepancy. The actual lane source HEAD, file
hashes, package directories, compiled files and symbols bind adapter validation;
matching embedded VCS revision or release provenance is not claimed. No shared
Git/tooling changes or stamp suppression were performed. WP12 must check intended
source/release metadata from its isolated committed integration tree.

Execution evidence is Linux amd64, Go1.26.4, Git2.43.0. No unexecuted OS is claimed
runtime-qualified. No live provider mutation, new module, SDK, provider CLI,
Python product dependency, mandatory service, source deletion, workflow execution,
public push/merge/tag or release publication occurred.

`raw.tar.gz` contains explicit selected text logs/scripts/source patch only.
Binaries, exported source fixtures, Git repositories and private HOME contents
are excluded. archive-members.sha256 binds inner members; archive verification
re-extracts and checks them. Outer SHA256SUMS binds all delivered evidence files.
