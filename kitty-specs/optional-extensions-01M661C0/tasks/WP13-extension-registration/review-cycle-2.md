---
affected_files: []
cycle_number: 2
mission_slug: optional-extensions-01M661C0
reproduction_command:
reviewed_at: '2026-10-31T00:18:33Z'
reviewer_agent: codex
wp_id: WP13
---

# WP13 independent review — changes requested

Reviewer: actual tool `codex`, resolved profile `reviewer-renata`. This is the first substantive WP13 code review; the generated feedback number reflects prior administrative allocation recovery, not an earlier code rejection.

Reviewed delivery `8f68d66af1aa00093e428e643625184f4958b17a`, tree `8b8094ac6de45059948752321814fe50a3c1f6d4`, and its six owned Go files. All 226 recorded Go/module input hashes match. Exact supplied Linux AMD64 binaries match SHA256 `4c83405e2c1cbcb5e7e718b1125b47be64e7775779f00301ef241b896081a2ee` (standard) and `cf7aaa4266b2882a202b61b21c4bf7ca824c77997118d6c6258c914e819cb404` (`nogithub,noforge`), both stamped with the delivery revision and Go 1.26.4. Subsequent lane commits change coordination material only. The primary host contract SHA256 is `6ff3466337cd6c3a6d74568ae8d851059b7d4d134a52c9f7c07f57a9a704a2a5`, including the independently approved four-replacement registration lifecycle erratum (`263fdae9`).

## Blocking finding

**[P2] Reject null argv elements before decoding the registry into Go strings.** In `internal/extension/registration.go:267-293`, `encoding/json` decodes a JSON `null` element of `argv` into the zero value of a Go string. The subsequent descriptor and identity validation sees `""`, so malformed input can pass validation using the identity of a valid empty argument. This contradicts the frozen UTF-8 **string array**, strict JSON, and malformed-file/no-implicit-repair requirements.

Independent real-CLI reproduction on **both exact supplied binaries**:

1. Register a valid manifest and operator-selected executable with one empty argument (`--arg ''`). Registration succeeds; the private registry contains `"argv":[""]` and the instance-bound registration ID.
2. Change only that array element in the stored registry to `"argv":[null]`, retaining the existing ID and every other field.
3. `hn extension inspect MODULE_ID --json` incorrectly returns exit 0, `ok:true`, `status:"registered"`, `availability:"not_probed"`.
4. Repeat the original register command. It incorrectly returns exit 0 and silently replaces the malformed registry with `"argv":[""]`.

The valid empty-string control succeeds with registry bytes unchanged. Boolean, number, object, and array element controls correctly return exit 1 / `host/invalid_input`, `do_not_retry`, `operation:null`, and leave their exact malformed bytes unchanged. Only the null control is coerced and repaired. No module marker is created. Detailed retained transcripts, actual input/output digests, and fixture bytes are in the external WP13 `reviewer/argv-type-controls.json` and `reviewer/null-argv-repro.json`; private registry contents are not committed here.

Required correction: validate the raw parsed `argv` array and every element's JSON string type before typed decoding or identity checks (or use an equally strict non-coercing decoder). Add a package regression and supplied-binary live CLI regression proving `[null]` refuses with the exact structured error and preserves the malformed file for inspect, duplicate register, and remove. Keep a valid empty-string argument positive control; do not prohibit valid empty arguments to hide the issue. Do not loosen any frozen contract or repair malformed state.

## Subtasks and contract round trips

- T052: strict manifest field/casing/duplicate/Unicode/capability/size tests and inert CLI checks satisfy the demonstrated scope.
- T053: physical executable identity, private storage, fresh instance, exact duplicate/no-entropy behavior, missing/malformed/mismatched instance, and retained old repository ENTRY are covered, but strict stored argv handling fails as described above.
- T054: live `main.go` fallback -> `localCommands["extension"]` -> `cmdExtension` -> the used static `extensionSubcommands` map is present. Native commands do not call registry access. Actual supplied-binary absolute and relative registry/enablement/declaration read controls pass.
- T055: live closed inspect/remove shapes, host/not_found, known-schema incompatible protocol, changed/missing executable, unsupported inventory, and not-probed availability are demonstrated. Malformed registry inspection still needs the correction above.

The registration preimage includes the private 32-lowercase-hex instance; active duplicates preserve the stored ID, remove/re-register produces a new ID, and a second repository's retained old ENTRY remains byte-identical. This review does not claim future WP14 enablement/run enforcement or WP16 finite session cleanup. The named inspection result and existing remove function are actual used seams for those later wrappers, not evidence that those later behaviors already pass. Public manifest/executable/registration digest round trips in the live CLI tests match the frozen canonical encoding and closed management envelopes; invalid instance cases retain their exact malformed bytes and return host-origin failures.

## Eight anti-pattern checks

1. **Dead code — PASS.** Production call-site inspection finds each new public behavior reached through the real dispatcher, manifest reader, registry load/update/register/remove, or inspection availability path. No helper-only future host API is claimed.
2. **Synthetic fixtures — PASS.** Behavioral tests invoke production parsers/registry functions and real supplied CLI processes. Stored incompatible/retained-ENTRY fixtures state their limited scope; they do not simulate a successful future host.
3. **Silent empty return — PASS.** Empty registry/modules/grants/session lists represent documented absent/disabled/no-session state; actual failures carry structured errors. The null coercion is the explicit blocking defect, not an approved empty success.
4. **FR coverage — PASS for mapped scope, with the blocking malformed-input gap above.** C-002/C-003 and FR-002/003/004/NFR-001/NFR-006 have meaningful assertions for inertness, local lifecycle, capability inventory, default isolation, and private-field exclusion. Full mission qualification remains pending.
5. **Frozen surface — PASS.** The authored delivery changes exactly the six owned Go paths; no signed native/memory golden, existing foundation source, contract, main.go, or dependency file is modified by WP13.
6. **Locked decisions — FAIL.** Null argv acceptance and rewrite violate strict stored string typing and malformed-state no-repair requirements.
7. **Shared-file ownership — PASS.** All six authored paths are exclusively assigned to WP13. Static integration seams have current production callers and documented downstream ownership.
8. **Production fragility — PASS.** Explicit malformed/unsafe/busy/missing paths return bounded sanitized errors; no new panic or bare failure path is present. Atomic replacement and no-follow/ancestor checks are used.

## Independent verification and limits

Both exact supplied binaries pass fresh focused race runs of `TestExtensionRegistrationCLI` and `TestExtensionNativeReadDetector` (standard 1.415s; native-only 1.372s). These exercise actual temporary Git repositories, public/private file snapshots, structured errors, process markers, and syscall traces with deliberate absolute/relative read controls. The authored `BASE..DELIVERY` six-file `git diff --check` passes. No full suite/build/performance rerun was necessary; author package race/vet/build results remain author evidence. Python architecture `no_coverage` is unavailable coverage, not a Go pass. No other WP, main branch, release, remote, platform matrix, adopter trial, or entire-mission acceptance is reviewed here.

**Verdict: REJECT / return WP13 to planned for the strict argv correction.** No production or test fixes were made by this reviewer. Preserve this failure and all original evidence; obtain a new independent verdict after the correction.
