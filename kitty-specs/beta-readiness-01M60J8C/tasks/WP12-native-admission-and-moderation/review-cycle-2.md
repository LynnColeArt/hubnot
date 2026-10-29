---
affected_files: []
cycle_number: 2
mission_slug: beta-readiness-01M60J8C
reproduction_command:
reviewed_at: '2026-10-29T13:32:36Z'
reviewer_agent: codex
wp_id: WP12
---

# WP12 follow-up feedback — P2 public interrupted-intake recovery

Review disposition: **request changes / reopen WP12** on newly demonstrated
WP13 public-acceptance evidence. Preserve the earlier approved review cycle and
its evidence as historical records; this feedback does not overwrite them.
Root has directed repair before beta qualification. WP13 is a dependent and must
integrate the approved repair before its final acceptance/full gate.

## [P2] Wire durable intake reconciliation into selected public retry

**Requirements:** FR-020 (portable lifecycle including interrupted recovery),
SC-011 (safe interrupted public retries), WP11 T037 (controller recovery), WP12
T042 (live exchange/reconstruction/safe retry), and WP13 T044/T045 (interruption
acceptance and actual operator recovery instructions).

**Trigger:** an ordinary supported signed artifact has been fully read, validated
and durably retained by the receiver. Interruption leaves its existing original
operation receipt reserved/uncertain before publication. The foreign source file
is then removed. Under legitimate selected authority, an operator repeats the
same `contribution receive` or `restore` operation/digest/exact-ref selection.

**Actual:** `ContributionExchange.Receive` (contribution_exchange.go:1189–1198)
returns the existing unchanged reserved/uncertain/failed receipt; the live handler
(contribution_commands.go:367–368) prints nonpending/nonconflict status and returns
success before retention/admission. No production caller invokes the existing
`ContributionExchange.Recover` (:1505). Exact public retries therefore remain
uncertain forever, although the original immutable retained artifact is available
and the controller API can reconcile it. Authority re-selection handles only
pending/conflict attempts and does not supply the missing public path.

**Concrete evidence:** WP13's source-assisted valid artifact-durable interruption
fixture, followed by the selected ACTUAL CLI retry before any API Recover call,
returns stable uncertain status, unchanged ledger/refs and no publication with the
source removed. Draft130805a8/focused-7 demonstrates this behavior in standard
and actual combined-native variants. These are passing observations of the current
limitation, not successful public interrupted recovery. Earlier focused-6 standard
batch also records it. External assessment:
`work/beta-readiness-WP13/interrupted-intake-assessment.md`.

**Expected:** selected public retry revalidates only the receiver's original
immutable retained bytes with its bound original digest, selection, reservation
and per-attempt limits; reconciles that SAME attempt without an extra input charge;
then follows the existing raw retention/current-authority admission flow appropriate
to receive versus restore/read-only. Missing/corrupt retained bytes must refuse and
remain charged/unpublished. A new operation that spends another input reservation,
journal deletion/reset, foreign-path reopening or an unsupported developer API call
is not an equivalent operator recovery path.

## Minimal repair direction and boundaries

Implement in WP12's declared command/integration-test ownership. Prefer the
existing selected exact receive/restore retry seam using EXISTING `x.Recover`
for recoverable public-artifact statuses, followed by normal existing retention
and current live authority/cut checks. No new public verb is needed unless a
concrete implementation reason is reported first. Review the internally generated
control-record `Receive` seam in `contributionStoreRecords` as well: retry of a
previously interrupted receipt/decision intake must not stay stranded just because
its controller receipt is not pending yet.

Keep `ContributionExchange.Receive`'s exact-identity validation, existing parser,
store, WP11 journal/schema, signature/native verifier, quota/accounting semantics
and original immutable history unchanged. Never spend a foreign receipt as local
capacity, refund/reset failed charges, infer missing evidence succeeded, adopt
source authority or grant a signing role. Preserve private local descriptor and
baseline/retained-registration distinctions; public recovery must not reinterpret
those intents as completed public artifact intake. Respect unselected restore and
keyless/read-only operation: reconciliation/retention cannot silently sign project
admission. Receipt generation remains separately charged and currently authorized;
its artifact/accounting/window/cost provenance must still bind the original attempt.

## Acceptance controls for the repair

1. Actual selected standard/native-only CLI retries a valid already-durable
   interrupted input with the original source removed and reaches the appropriate
   pending/admitted/raw outcome. The source fixture is explicitly labelled as
   deterministic interruption setup, not a real process crash/cancellation claim.
2. Repeat exact retry and compare original IDs, payload/signatures, ref OIDs and
   receiver-local ledger: no duplicate native writes or new input reservation.
   Account separately for any authorized new receipt-control reservation.
3. Missing/corrupt retained data, wrong digest/ref selection and private-descriptor
   alias refuse before unintended promotion and preserve existing charges. Original
   reserved limits/selection remain binding across policy/window changes; any new
   admission attestation uses current legitimate authority.
4. Cover receive, selected restore/read-only and unselected restore semantics, and
   generated control-record interrupted retry. Unsupported incomplete input remains
   an explicit refusal, not a guessed successful recovery.
5. Update the downstream WP13 guide/regression to the real supported public
   behavior after this upstream repair. Do not count the old forever-uncertain
   demonstration or controller-only Recover test as public recovery acceptance.

Run focused controls first, then coordinate the single full-suite slot with root,
follow normal commit/for_review and independent fixed-head review. Do not change
WP13 status, broaden ownership or bypass framework gates to land this correction.
