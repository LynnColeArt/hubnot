---
affected_files: []
cycle_number: 1
mission_slug: optional-extensions-01M661C0
reproduction_command:
reviewed_at: '2026-10-31T00:05:47Z'
reviewer_agent: codex
wp_id: WP03
---

# WP03 review cycle 1 — changes requested

Actual reviewer: codex / reviewer-renata. Reviewed source `cecac4032684a6a37385312ce06173826227b2c7`; delivery `15417121c112da89ce6db3aff911139b1a45b668`; actual claim HEAD `3cf1a26d4aba4a6c3457b3da4c0b0257dbd7f387`. All 227 source-input hashes match the bound binaries. This rejection does not weaken or expand the frozen artifact/host contracts.

## R1 [P2] Keep envelope refusals within the closed artifact-origin error set

**Code:** `internal/artifact/envelope.go:53-60`, `artifact_commands.go:45`. **Contracts:** PRIMARY `contracts/host-v1.md:493-497` (closed artifact-origin set), `contracts/artifact-v1.md:363-365,382` (CLI uses that frozen error wrapper).

A frozen signed `unknown-protocol` input returns `origin=artifact, code=incompatible_version`; a freshly signed unknown-kind input returns `origin=artifact, code=unsupported_capability`. Both codes belong only to the frozen host-origin set and are absent from the artifact-origin set. `artifactOutput` blindly exposes them, and the present unit/public CLI tests explicitly assert the wrong protocol code. A strict independent consumer therefore receives invalid error objects despite exit 1.

**Reproduction:** `python /home/lynn/Documents/Codex/2026-10-24/wa/work/optional-extensions-WP03/reviewer-reproduce-error-codes.py` uses each exact supplied binary and public contract literals. Actual responses for both variants are retained in `reviewer-focused-reproduction.json`.

**Required correction:** Refuse these unsupported signed-envelope protocol/kind literals as `artifact/invalid_input`, with distinct sanitized protocol/kind diagnostics, `recovery=do_not_retry`, `operation=null`, `data=null`, exit 1. Preserve the closed host/artifact origin separation; do not expand the artifact code set or relabel artifact input as host negotiation. Update the existing wrong-code assertions and add a public assertion that emitted artifact-origin codes stay within the frozen set, including unknown protocol AND unknown kind. The opaque-preservation behavior of later store/transport slices remains separate and unclaimed here.

**Acceptance:** Both exact corrected binary variants return the permitted origin/code/error shape for these controls, all prior valid vectors still verify with identical IDs/bytes, and signature/canonicality failures retain their intended refusals.

## R2 [P2] Permit predecessor-only stdin under the frozen FILE grammar

**Code:** `artifact_envelope_commands.go:33-34`. **Contracts:** PRIMARY `contracts/artifact-v1.md:348` (`FILE` local input and `-` stdin), §7.1 lines 373-380 (paired predecessor and two simultaneous stdin refusal). Root explicitly confirmed the existing FILE macro applies to either payload, with only two simultaneous stdin selections forbidden.

The condition `previousInput == "-"` rejects every predecessor stdin selection, even when the current envelope is a file. With current `type-list-v1` in `second.json`, stdin containing its signed `type-note-v1` predecessor and both matching signatures, both supplied variants exit 1 with `artifact/invalid_input: predecessor requires a file and signature`. The contract requires exit 0 and `continuity=predecessor_verified`.

**Reproduction:** Exact argv, public input hashes/signatures and responses are in external `reviewer-predecessor-stdin-reproduction.json`; fixture files are retained in the directory recorded there. The current equivalent file-predecessor/current-stdin path succeeds. Double stdin refuses even with empty stdin, as it should.

**Required correction:** Keep paired-flag validation, but reject stdin only when BOTH current and predecessor input select `-`; permit a single stdin source in either position through the existing bounded input reader. Refuse double stdin before consuming either stream. Add actual built-process positive cases for predecessor-stdin/current-file through validate and inspect, retain current-stdin/predecessor-file positives, and a deliberately unreadable/blocked-or-empty stdin double-selection negative proving upfront `invalid_input`. Keep each payload's independent 1MiB bound; verify predecessor-first failure and first-over predecessor do not create public/extension state. Do not narrow or amend the frozen CLI grammar to fit the implementation.

**Acceptance:** Both corrected exact binary variants produce the exact existing success fields (inspect adds only envelope), `predecessor_verified`, correct current ID, and unchanged semantic_validation=not_checked for a valid predecessor-only stdin pair; malformed/missing flag pairs, wrong predecessor and double stdin retain precise refusals.

## Evidence retained and fix scope

Independent narrow reproduction covered 28 cases plus four stdin-direction cases on both bound binaries. Full temporary repository/private-file inventories were unchanged in the first 28-case run. No reviewer app source fix, build or broad test rerun was performed. Canonical frozen public vectors in the Go tests match all 25 published byte/signature/ID/preimage literals exactly. Author focused race/native-memory goldens/vet/build and truthful type128KiB dominated-ceiling evidence remain accepted for unchanged behavior; the Python gate's no_coverage is not a Go test claim. Preserve initial-red and unsafe0755-fixture diagnosis evidence.

Make only the small owned-source/test corrections, rerun the affected focused CLI/artifact checks and both rebuilt exact binary controls, bind the new inputs/binaries and use the normal for_review gate. No duplicate full suite is requested. WP04 and WP05 depend on WP03 and must remain unclaimed until this correction is independently approved; root coordinates any dependency refresh. Final platform/adopter/performance/full-mission gates remain pending.
