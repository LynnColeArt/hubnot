# Interrupted operations and upgrades

This guide implements the prospective [beta support contract](beta-support.md).
Current releases remain alpha. A journal schema is a storage contract, not a
product release number. There is no blanket promise to resume every historical
alpha journal with a newer binary.

Before upgrading, finish pending operations when possible. Keep the exact
originating compatible executable, its checksum, product version/source revision
and build variant, together with an operator record of the original command,
repository, signer, session directory, destination and API profile. Record this
outside the journals: the formats below generally do **not** record the product
release that created them. Never infer an originating release from `/1` or an
integer version. Keep private backups protected; do not commit journals, keys or
credentials. A public project archive is not a session or private-state backup.

An upgrade must explicitly support a pending session or leave it intact for
recovery with its originating compatible binary. An unknown version, unsafe
file, invalid authentication or conflicting progress is an error, never a new
empty operation. Do not edit version fields, remove checkpoints, rebind a
destination, copy a session into another project, or downgrade a journal to make
it open. Newer signed facts may also make binary rollback unsupported. Retaining
an old binary does not promise it can read history written after the upgrade;
keep the original session and inspect the compatible recovery path before
allowing new writes. See [private disaster recovery](recovery.md) for signer
fencing and accepted-history reconciliation.

## Current inventory

Paths beginning `hn/` are relative to the resolved Git directory, usually
`.git/hn/`, rather than the worktree root. Use the original repository and exact
selection; journals bind intent and progress but grant no provider permission,
native role or audit authority.

| Operation | Retained state and current format | Recovery command and upgrade boundary |
| --- | --- | --- |
| Migration engine/CLI | Explicit private session directory: HMAC envelopes for `plan.json` (`hn.migration-plan/1`), `snapshot.json` (`hn.project-snapshot/1`), `progress.json` (`hn.migration-progress/1`), `cli.json` (`hn.migration-cli/1`), random `key`, immutable `blobs/`, and `apply.lock` | `hn migrate inspect --session DIRECTORY`, then `hn migrate resume --session DIRECTORY --confirm-apply` with fresh explicitly selected credentials. Preserve all files and the saved destination/API profile. Missing progress is not an empty session. No arbitrary alpha-session migration is promised. |
| Issue transfer into native history | `hn/forge/native-intent.json`, `hn.forge-native-intent/1` | Repeat the original `hn transfer import --provider PROVIDER --project PROJECT --key KEY` and its exact endpoint/options. The intent binds actor, operation and payload; signed native operations recover a lost acknowledgement. |
| Typed resource transfer into native history | `hn/forge/resource-import-*.json`, `hn.forge-resource-import/1` | Repeat the original `hn transfer import` with the same `--resource`, provider, endpoint, project, key and mappings. The saved graph, original anchors, exact pending mutation and signed progress govern recovery; the source is not refreshed while recovery is pending. |
| Issue transfer out | `hn/forge/issue-export-*.json`, `hn.forge-issue-export/1` | Repeat the original `hn transfer export FULL_NATIVE_ID` with identical provider/project/options and the original active signer. The complete canonical journal is signed; unsigned older journals require explicit investigation with a compatible binary. |
| GitHub discussion/project transfer out | `hn/forge/discussion-export-*.json` (`hn.github-discussion-export/1`), `project-export-*.json` (`hn.github-project-export/1`) | Repeat the original export with the same resource, native ID, endpoint, destination/category and mappings, using the original signer. Do not reinterpret saved target IDs or replace the saved snapshot. |
| GitHub workflow transfer out | `hn/forge/workflow-export-*.json`, `hn.github-workflow-export/1` | Repeat the original workflow export with the same native snapshot, destination branch/path and explicit mappings. Its guarded blob-write intent does not enable native execution. |
| External transfer write inside an export journal | Embedded `hn.forge-remote-intent/1`: `prepared` → `in_flight` → `confirmed` | The enclosing exact transfer performs reconciliation. A timeout, negative scan, deletion, ambiguity or inaccessible object leaves uncertainty; none authorizes another create/PUT. Preserve the checkpoint for manual investigation if exact positive proof is unavailable. |
| Planned identity rotation | `hn/rotation-command.json`, integer version 1, containing the exact target identity and version-1 state; `hn/rotation.json`, integer version 1; identity records version 1; `active` pointer | Repeat `hn identity rotate`. The journals retain the same target key and predecessor. Validate saved state and signed authorization/acceptance progress before recovery writes. Only a verified complete pair allows switching the active pointer. The target inherits no policy role. |
| Legacy identity layout migration | Existing `identity.json`, version-1 identity record and `active` pointer | An identity-loading command completes the existing durability sequence using the same actor/key. Keep the compatible binary and private files together; do not invent a separate session schema. This is layout recovery, not authorization to share a signer across devices. |
| Audit request/follow-up | `hn/audit/requests/ACTOR/OPERATION_HASH.json` and `followups/ACTOR/OPERATION_HASH.json`; bounded private exact-intent JSON with no standalone version field; signed requests/issues carry portable operation proof | Repeat the original `hn audit request ... --operation OPERATION` or `hn audit follow-up RESULT --finding KEY --operation OPERATION --json`. Retain the original result/selection and actor. An omitted operation uses the command's documented default, not a new recovery key. Changed intent refuses. |
| Native merge/manual repair and automerge | Ordinary Git code/ref state plus signed proposal, decision and `proposal.merged` facts; no separate replay journal | Inspect current facts and target, then `hn merge FULL_CANDIDATE_ID`. A unique qualifying existing code merge permits only its missing signed fact to be repaired; current evidence and authority are rechecked. Repeating automerge after target movement does not authorize a second merge. |

Details remain authoritative in [migration engine](migration-engine.md),
[migration CLI](migration-cli.md), [transfer recovery](github-interoperability.md#private-recovery-and-publication),
[identity](identity-v0.md), [audits](audits-v1.md) and
[merge recovery](governance-v0.md#merge-and-conflict-recovery).

## Recovery procedure

1. Stop competing writers. Preserve the private session/journal and public refs
   before investigation. Inspect the owning process before removing a stale
   `apply.lock` or `transfer.lock`; a lock's existence alone is not proof that
   its process died. Never delete an uncertain write checkpoint.
2. Use the exact saved session and original selection. Inspect migration plans
   with `hn migrate inspect --session DIRECTORY`. Obtain fresh credentials only
   for the bound destination. Never treat source provenance or a journal
   signature as evidence of destination success or current native authority.
3. Retry the documented command with a compatible binary. Confirmed progress
   stays confirmed; dependent writes use the exact parent receipt. In-flight
   remote writes require one exact positive reconciliation. If proof is missing
   or ambiguous, preserve state and inspect remote outcomes manually. Starting
   another session does not cancel an earlier possible remote creation.
4. Check the actual public result and retained history. Migration spans APIs,
   Git objects, archives and refs without a global transaction: a failed apply
   can leave partial archive/object installation. A failure is not a claim that
   nothing happened. Audit readers can materialize embedded snapshot objects;
   ordinary inspection is not universally free of object-database effects.
5. After successful recovery, retain the compatible reader and protected backup
   required by the upgrade plan. Do not assume an older binary can read newly
   published facts. Public recovery and exclusive signer restoration remain
   separate procedures.

## Executable evidence and limits

The offline tests use disposable repositories and identities; they never read
the operator's keyring. They establish the named local boundaries, not live
provider qualification or distributed exactly-once publication.

| Boundary | Executed regression surface |
| --- | --- |
| Authenticated future migration plan/progress/CLI metadata refuses with session and destination unchanged | `TestBetaMigrationFutureSessionRefusesWithoutReset` reseals a future schema with the disposable session's real HMAC; it proves version/binding refusal rather than a bad-MAC failure. |
| Future native import, resource import, signed issue export and embedded remote intent refuse without resetting progress or adding native/provider writes | `TestBetaTransferFutureJournalRefusesWithoutReset`; export fixtures are signed by the disposable original exporter. This table is not a promise to translate older unsigned journals. |
| Rotation reconciliation validates before writing; missing target plus future/unsafe state cannot recreate private material | `TestBetaRotationJournalRefusalPreservesRecovery`, standard and native-only public CLI subprocesses, complete repository file/mode snapshots including private state, refs and objects. The fixtures contain no cold audit packs; see the inspection materialization boundary above. |
| Rotation crash points, exact target reuse, signed event verification and active pointer/cleanup durability | `TestIdentityRotationRetriesWithoutSiblingFacts`, `TestIdentityRotationReusesTargetAfterRecordDurabilityFailure`, `TestIdentityRotationCommandRetriesCleanupBoundariesWithoutSecondRotation`, `TestIdentityRotationRejectsHostilePersistedEventState`, `TestIdentityRotationResumesVerifiedCompletedPair`, and the `TestRotation*` keyring interruption suite. |
| Migration lost response, parent binding, missing checkpoint and negative lookup | `TestMigrationPersistedCommentBindingAndNegativeLookupNeverRetries`, `TestMigrationMissingCheckpointNeverReplaysCreates`, `TestMigrationNativeLostAckReconcilesExactRefs`, and `TestMigrationCLIGitLabToGitHubGuidedResumeAndOffline`. |
| Transfer saved source, positive-only remote reconciliation and nonduplicate creates/PUTs | `TestResourceBridgeRecoveryDoesNotRefreshSource`, `TestForgeExportUncertainNegativeScanNeverRepeatsPOST`, `TestGitHubDiscussionUnknownCreateDoesNotRetryAndTamperedPlanRefuses`, `TestGithubProjectNegativeRecoveryNeverRetries`, `TestGitHubWorkflowUnknownNegativeResultNeverRetriesPUT`. |
| Audit request/follow-up lost acknowledgements preserve exact operation and later issue edits | `TestAuditPublicRequestConcurrentOperationAndLostAcknowledgement`, `TestAuditPublicFollowUpInterruptedCreateRecoveryAndEditedIssue`, `TestAuditFollowUpUnsafePrivateIntentRefuses`. |
| Code merge precedes event, retry repairs once and stale evidence refuses | `TestMergeRetryRepairsMissingEventAndIsIdempotent`, `TestShallowMergeRetryRepairsMissingEventWithAppendCAS`, `TestMergeRetryFailsClosedWhenCurrentEvidenceChanged`. |

These tests exercise current supported formats and explicit future refusal.
They do not identify the originating product release from a journal or qualify
every alpha format. Keep unexecuted provider/platform observations pending in
the beta evidence matrix.
