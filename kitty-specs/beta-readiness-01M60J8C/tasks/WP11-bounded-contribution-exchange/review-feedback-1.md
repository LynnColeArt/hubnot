# WP11 review: changes requested

Reviewer: reviewer-renata / codex. Reviewed source: `6207ff2122518a61f11178df012d36bf8dd134c5`.

## P1 — Persist and charge the complete raw-retention intent before immutable object writes

`contribution_exchange.go:2089-2100` calls `prepareContributionManifest`, which writes payload/signature/manifest blobs, a tree and a commit, before persisting either the receiver destination (`j.GitDir`) or this actor's expected-old/material plan (`ManifestWrites`). Any error or interruption before that journal save leaves real retained objects but no durable binding. Deterministic commits protect only a retry with identical destination and expected head; the same intake reservation can be retried against other destinations, or another prior head, and retain additional immutable material without another charge.

The independent production-path probe reserves one intake with `Objects=16`, fails at the existing `manifest-objects-durable` seam, and retries the same intake/actor into four ordinary bare Git directories. It retains **20 actual Git objects**, while the ledger still has **one request**, a **16-object reservation**, and **zero manifest plans**. All four writes return failure and no raw ref is promoted. This violates T036/T037, FR-017 and NFR-006: failed and uncertain retention must remain bounded and charged, and destination changes cannot reuse capacity.

Repair: durably bind the immutable destination, actor, exact expected-old head, selected signed material and retention operation/charge before the first object write. Replays must resume that exact pre-write plan; changed destination/head/material must refuse before mutation or require a separately reserved operation. Preserve existing failed/uncertain ledger history. Account for all immutable material left by failed attempts and prevent repeated failures from spending the same reservation again. Merely moving `j.GitDir` earlier leaves the changed-expected-head variant unresolved.

Add a real regression exercising interruption before the current plan save, process reopen, changed destination and changed expected head. Assert actual retained object/byte totals against durable charges and zero additional writes on refused retries, while preserving a successful exact retry and raw-ref CAS recovery.

Evidence:
- `/home/lynn/Documents/Codex/2026-10-24/wa/work/beta-readiness-WP11/evidence/review-preplan-destination-probe_test.go`
- `/home/lynn/Documents/Codex/2026-10-24/wa/work/beta-readiness-WP11/evidence/review-preplan-destination-probe.log`

The existing full race/static evidence is valid for this source; the independent focused race rerun also passes. Those positive results do not cover this failing boundary. Keep this review scoped to the declared regular-file library exchange; WP12 live authority/admission and WP13 public CLI proof remain mandatory. Notify/rebase dependent WP12 after repair.
