---
affected_files: []
cycle_number: 1
mission_slug: optional-extensions-01M661C0
reproduction_command:
reviewed_at: '2026-10-31T01:44:39Z'
reviewer_agent: codex
wp_id: WP05
---

# WP05 review feedback 1 — codex / reviewer-renata

**Issue 1 [P2]: Reject incomplete immutable intents before the current writer signs or publishes.**

`artifact_store.go:209-243` decodes the private descriptor directly into `artifactIntent`. The shared struct decoder rejects unknown fields but does not require field presence. Removing `expected_oid`, `candidate`, and `result` from an otherwise correctly bound `prepared` descriptor produces nil defaults, passes the stage checks, and is overwritten by `appendArtifact` before it signs and publishes a new object. This violates `contracts/artifact-v1.md` section4, which requires exactly eleven fields, explicit nullable witnesses, and complete immutable publication evidence.

Independently reproduced against delivery source `40e1335f2c7e232ba409be2a4e8d417e5e773a5a`, supplied standard binary SHA256 `b633f18d1b406b641e70ca3716ae5ec1414740014d681a53f6d3f1a7cdf8c55b`: initialize a disposable native identity/repo, publish a valid type, create another correctly actor/key/request-bound prepared descriptor from the valid descriptor with the three required nullable keys omitted, then invoke the exact current `artifact type publish` input/actor/key. Expected `artifact/verification_failure/do_not_retry` with unchanged intent/public objects/ref; actual exit0 success, a fresh signed event and changed public actor ref. Raw disposable fixture/evidence remain outside Git under `optional-extensions-WP05/reviewer-independent/` (`missing-required-intent-fields.json`, `malformed-prepared-input.json`). No private keys are in this feedback.

Require bounded strict descriptor validation that distinguishes missing fields from explicit null, validates the exact required shape before signer/public effects, and refuses duplicate/unknown fields and invalid nullable field types. Add meaningful current public mutation negative controls for malformed existing intents, including each omitted required nullable key; compare complete pre/post public/private snapshots. Preserve exact2MiB valid intent acceptance and one-over refusal. Do not weaken the existing envelope/schema bounds or change inherited approved files. This is current WP05 retry consumption, independent of future WP07 operation lookup/reconciliation scope.

Other reviewed controls passed: full focused standard race20 top-level/32 test events (10.680s), native targeted supplied-binary subset6 pass (4.860s), actual19MiB accepted-history read, retained16MiB utility cap, independent live and literal Ed25519/hash/ref/Cut recomputation, policy/CAS/no-fetch/corruption/native-history controls. These passes do not waive Issue1. No fixes were made by the reviewer.
