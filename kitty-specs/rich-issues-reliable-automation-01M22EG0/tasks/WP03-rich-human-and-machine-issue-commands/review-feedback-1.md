# WP03 independent review — changes requested

Reviewer: codex-independent-cli-review, resolved Reviewer Renata profile.
Reviewed source: `c0b6f6d922cd876dc58793beade4182f6fada4d8`.
All four owned files remain byte-identical to that commit during review.

## P2 — Use consistent machine-mode parsing for failure output

`issue_commands.go:86-99` recognizes only a subset of boolean spellings, while `parseIssueOptions` delegates `--json` to Go's flag parser. Successful commands use the parsed `options.json`, but errors use the narrower `issueMachineRequested` result at lines 121-129.

Consequently, the public command `hn issue list --json=1 --limit 0` exits nonzero with empty stdout and only `hn: invalid_input: page limit must be 1..200` on stderr. `--json=T` and `--json=TRUE` reproduce this, while `--json=true` produces the required JSON envelope. These are accepted true values for this command's flag parser, so a consumer receives machine output on success and no machine response on failure with the same flag.

Make machine-mode interpretation consistent for all accepted boolean spellings and repeated true/false flags, including failures during parsing. Preserve the `--` delimiter and distinguish option values/free text from actual output flags. Add public subprocess assertions requiring one error envelope and a nonzero exit for accepted true forms. Explicit false forms should consistently retain human mode. Keep unsupported/malformed flags deterministic; a broad CLI rewrite is unnecessary.

Requirements: FR-008 and T008's exactly-one-machine-response contract.

## P2 — Reject malformed Unicode before signing repaired input

`readIssueState` at `issue_commands.go:338-370` uses `encoding/json` tokenization and decoding. That decoder replaces invalid UTF-8 bytes and unpaired escaped UTF-16 surrogates with U+FFFD. `CanonicalIssueState` then sees valid replacement-character strings, so its UTF-8 validation cannot detect the original corruption.

Through the actual binary, submitting either `{"title":"before<raw byte FF>after"}` or `{"title":"before\ud800after"}` to `hn issue open --input - --json` returns success and publishes a signed issue instead of rejecting the invalid input. This silently changes submitted title content before creating immutable history; malformed metadata keys or values can be similarly reinterpreted.

Validate raw UTF-8 and escaped surrogate pairing before the lossy decoder conversion, or use an equivalent strict approach within the owned input boundary. Return `invalid_input` and leave refs unchanged for malformed input. Preserve valid Unicode, properly paired surrogate escapes and intentional U+FFFD content. This request concerns new CLI JSON input only; do not retroactively change legacy signed-event decoding or broaden the protocol migration.

Requirements: FR-002's lossless state, FR-008's strict machine input, and T008/T009's validated canonical content boundary.

## Reproduction and verification evidence

The reviewer added no implementation or test files to the worktree. The independent tests are supplied through a Go overlay under `/home/lynn/Documents/Codex/2026-09-09/h/work/wp03-review/`.

```bash
go test -overlay /home/lynn/Documents/Codex/2026-09-09/h/work/wp03-review/overlay.json -run '^TestReviewWP03StrictPublicBoundaries$' -count=1 -timeout=60s -v .
```

Run from the WP03 lane worktree. This fails in 0.348s: the standard `--json=true` case passes, the three additional accepted true spellings omit their error envelopes, and both malformed-Unicode cases report success and change signed actor refs. The tests invoke the real built CLI using stdin and assert process status, stdout and ref changes. Add production regression tests first, then make the narrow fixes and rerun this independent reproduction.

Existing focused independent verification:

```bash
go test -race -run '^TestIssueCommands' -count=1 -timeout=90s -v .
```

PASS, 9.956s. Covers replayed close after reopen, staged snapshot replay and 205-head convergence, actor-local CAS loss, cross-actor conflict preservation, divergent/duplicate operation keys, pagination/stale cursors, graph provenance/cycle pages, strict duplicate/unknown/null input and public exit/stdout tests.

`git diff --check` passes. The implementing agent's saved full race log reports 215.648s and focused race log 11.862s; full regression was not duplicated without a new whole-suite concern.

## Required anti-pattern checklist

1. Dead code: PASS. `run` calls `cmdIssue`; its mutation/query paths call the reader, catalog, candidate validation and live rendering helpers. Compatibility wrappers retain their prior callable surface.
2. Synthetic-fixture tests: PASS. Existing tests exercise real production commands and signed histories; deterministic race seams use actual append/CAS. Independent failures exercise the built CLI.
3. Silent empty return: FAIL at the machine-error boundary described above; accepted machine mode can fail without any stdout response.
4. FR coverage: FAIL for the strict/lossless and machine-failure boundaries above. Existing tests otherwise demonstrate the main mutation, conflict, replay and navigation journeys.
5. Frozen surface: PASS. The WP source commit changes only its four owned files; no immutable historical evidence changed.
6. Locked decision: PASS for signed repository authority, explicit heads, descriptive state and no global-CAS claim; input/output contract defects are separately recorded above.
7. Shared-file ownership: PASS. All four implementation/test paths belong to WP03; dependency sources are unchanged. Inherited coordination metadata is centrally tracked and was not removed or restored.
8. Production fragility: PASS. Stale/conflict/resource failures are deliberate diagnostics; no new panic or implicit execution authority was found. Correct the two response/input boundaries without weakening those guards.

Verdict: reject pending the two bounded interface fixes and their regression evidence.
