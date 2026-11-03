# WP09 independent review — changes requested

Reviewer: `codex`, resolved profile `reviewer-renata`, 2026-10-31 UTC.
Reviewed delivery `ebc5074994cfa0b025ee25630a38125dd509db3e`; review allocation
HEAD `8d83d2c014640b61b376689571b6fefcb6e53634` has identical 254 Go/module inputs
(map SHA-256 `0d2e78d9f91ddfc14e3afdf4182107dc6b75df71aac96f55876bcec737e51407`).

## Issue 1 — copy verified accepted Git bodies without imposing a new metadata restriction

**Priority P1; location:** `artifact_replication.go:257`, `copyArtifactInventory`.

The frozen conformance fixtures explicitly identify `type-note-v1` and
`optional-missing-target` as accepted public histories. Their exact Git commit
bodies and OIDs are literal in `contracts/conformance-fixtures.md` (lines82 and682).
Artifact contract sections1 and5 require the same signed identities and selected
raw Git history to remain portable; Git author/committer metadata is transport
metadata, and transport consumes the shared verified closure. WP09/FR-014 requires
independent public reconstruction without source private state.

Both supplied standard and combined-native binaries successfully inspect these
exact histories as verified, including the exact schema and `missing_optional`
diagnostic. Push then refuses `artifact/verification_failure`, message
`artifact object copy failed verification`, before publication. The copy invokes
`git hash-object -w -t commit --stdin`, which applies an additional Git fsck
author-email restriction. The accepted fixture bodies use `Fixture ACTOR@hn.invalid`
without email angle brackets; Git reports `missingEmail`.

This is isolated from signatures/schema/closure: independently recomputed
Ed25519 signatures, artifact IDs, and raw Git hashes all match the frozen literals.
A comparator changes only unsigned author/committer formatting to
`Fixture <ACTOR@hn.invalid>` and consequently Git commit OIDs, preserving every
signed payload/signature/artifact ID. That comparator completes public push/fetch
with both supplied binaries. No frozen file was edited, and neither source nor
destination ran `hn init` or had a local signer.

**Reproduction:** run the external preserved reviewer script with either supplied
binary, without `--valid-metadata-control`:

```sh
python3 /home/lynn/Documents/Codex/2026-10-24/wa/work/optional-extensions-WP09/review/public_literals.py /home/lynn/Documents/Codex/2026-10-24/wa/work/optional-extensions-WP09/std-hn
```

The already-created exact-literal repository is
`/home/lynn/Documents/Codex/2026-10-24/wa/work/optional-extensions-WP09/review/wp09-review-egy599q0/src`.
Structured outputs for both binaries are in `review/literal-copy-refusal.json`;
all literal/comparator raw bodies and hashes are preserved in
`review/literal-and-comparator-objects.json`. The comparator is explicitly labeled
as a cause-isolation control, not a substitute for passing the frozen literals.

**Required correction:** copy the already verified accepted raw object bodies
byte-for-byte, retaining their exact OIDs and existing verification/resource/CAS
guards. Add an actual public Git transfer regression using these accepted literal
commit bodies, proving public reconstruction on an independent destination with
the exact raw objects and IDs. Do not normalize Git metadata, rewrite signed data,
change frozen fixture/contracts, weaken malformed-history/signature/native
refusals, or claim the comparator as the corrected case.

## Verification and contract round trips

1. **FAIL:** frozen accepted envelope/signature/Git-body literals → independently
   recomputed identities → successful source inspect → failed exact-body push,
   both supplied package variants. This is the single blocking finding.
2. **PASS comparator:** public closure → real bare remote → independently initialized
   destination without signer/private state → exact signed IDs and returned Cut
   equal to current inspect. Missing required schema and invalid existing destination
   projection both refuse before destination object copy or accepted refs.
3. **PASS:** frozen expected-old/policy/uncertainty contracts → fresh current focused
   race checks, including competing-ref preservation, logical conflict/all heads,
   list order, selected-policy refusal before remote lookup and dispatched-push
   uncertainty. Focused run passed in9.492seconds.
4. **PASS:** fixed custom ceilings/native selection decision → fresh literal boundary
   and native lower-budget/private-file-independence controls. Prior physical
   depth256/257 and independent equivalent-wire16777216/16777217 evidence was
   inspected and retained, not repeated. Bounds masked by earlier limits remain
   labeled numeric stage controls. Existing115 native compatibility events had
   zero failures/skips. Final full uncached qualification remains WP25.

The fetch response's empty transaction diagnostics is **not a finding**: section7
does not explicitly require aggregation there. Current inspection of the preserved
signed links emits `missing_optional`; the conflict/list CLI comparison retains
the corresponding record diagnostics.

## All eight anti-pattern results

- Dead code: **PASS** — static registration reaches the actual CLI transport and
  all helpers have current production callers.
- Synthetic-fixture tests: **PASS** — existing cases invoke actual production
  entry points; the independent public oracle reveals the missing accepted case.
- Silent empty return: **PASS** — no hidden failure-to-empty path identified.
- FR coverage: **FAIL** — FR-014 exact public transport fails the accepted literal
  case above; this is the same Issue1, not an additional finding.
- Frozen surface: **PASS** — no contract/corpus/module input changes; reviewer
  preserved frozen files and all source/private-state boundaries.
- Locked decision: **PASS** — no additional native authority/default/budget
  violation identified; the exact-body portability gap is captured once above.
- Shared-file ownership: **PASS** — only the owned replication pair,
  `quarantine.go`, and replication docs changed; `replication.go` is unchanged.
- Production fragility: **PASS** — explicit failures and uncertainty handling;
  no detached local writer observed.

Full evidence report:
`/home/lynn/Documents/Codex/2026-10-24/wa/work/optional-extensions-WP09/review/review.json`,
SHA-256 `9f4e4cb78fa3d44e8bbb0313425b592916db9fff6985eb79c9683c5a453ee847`.
All runtime processes exited and the shared slot was explicitly released.
