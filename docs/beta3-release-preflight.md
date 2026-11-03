# Beta.3 release preflight — November 3, 2026

This is the contribution/fix review before freezing the `0.1.0-beta.3` candidate.
The requested deliverable is an **unpublished draft**, Linux AMD64/ARM64 and
Apple Silicon, standard and native-only. Intel Macs are outside this target set.
The release source/bytes are bound by the eventual `PROVENANCE.json`; this
document does not confer a runtime pass or adopt the full beta support contract.

## Included fixes and checks

| Item | Decision |
| --- | --- |
| [#26 / PR #32](https://github.com/LynnColeArt/hubnot/pull/32), receipt growth | Already merged and independently reviewed before freeze. Include delta acceptance receipts, preserved full-closure reconciliation and receipt-ID collision refusal. Old oversized receipts are not compacted. |
| [#28](https://github.com/LynnColeArt/hubnot/issues/28), invalid proposal ancestry | Fix before freezing: open/revise must prove base-to-head ancestry before signing or creating proposal refs. Use exact stored parents, with explicit missing-history refusal, so shallow markers do not create false negatives. Do not rewrite already signed invalid histories or add a destructive unpublished-event reset. |
| [#29](https://github.com/LynnColeArt/hubnot/issues/29), silent runner sync | Include UTC timestamps for runner status/errors and explicit sync start/completion/failure with elapsed time. This does not change scheduling, authority, pipeline execution or automatic defaults. |
| [#31](https://github.com/LynnColeArt/hubnot/issues/31), unpublished request ambiguity | Include an explicit local-only/publication reminder after `run request`. Do not add automatic sync/network access or alter signed request bytes. |
| [#23](https://github.com/LynnColeArt/hubnot/issues/23), Git-version-sensitive corruption regression | Already corrected at `eb3f4932`: probe the corrupt object and explicitly exercise physical inventory rather than depending on a missing-promisor fallback on every Git version. Retain corrupt-data refusal. Recheck locally and include the named regression in Pink Menace's native harness brief. No new corruption acceptance is inferred from the old failed fixture. |
| Apple Silicon-only Mac packaging | Use an explicit three-target packager selection. Teach the test-only bundle and extension-reference verifiers an explicit expected target set, preserving the original four-target default and full matrix/variant/checksum checks. Forward that selection into the missing-reference negative control. No Intel assets or Intel runtime promise in this candidate. |

## Deferred with rationale

| Item | Reason / boundary |
| --- | --- |
| [#27](https://github.com/LynnColeArt/hubnot/issues/27), obsolete requests / withdrawal | Native withdrawal events and skip semantics need a separate lifecycle/authority design. No cancellation protocol is introduced in a packaging fix. Document that the current runner may still spend work on obsolete requests. |
| [#30](https://github.com/LynnColeArt/hubnot/issues/30), duplicate work across runners | Cross-runner claims/leases and enough-results scheduling need explicit concurrent/recovery semantics. This candidate does not promise queue load-balancing or exactly-once execution. |
| [#21](https://github.com/LynnColeArt/hubnot/issues/21), memory operation recovery | A versioned signed-memory write/replay/lookup contract is substantial protocol work. Do not imply issue/artifact operation guarantees apply to memory writes or retry an uncertain memory append blindly. |
| [#20](https://github.com/LynnColeArt/hubnot/issues/20), standing acceptance / optional hosting | Separate architectural work; no implicit standing acceptance, server, default gate, service or additional authority is adopted for this release. Already merged requester/sync changes retain their existing scope. |
| [#24](https://github.com/LynnColeArt/hubnot/issues/24), older Mac certification | Preserve the old candidate's evidence/failures. Developer ID signing/notarization remain accepted follow-up work; browser/Gatekeeper success is not implied by terminal runtime. |
| [#33](https://github.com/LynnColeArt/hubnot/issues/33), beta.2 results | Historical exact-byte evidence, not transferable to beta.3. Obtain a fresh native Apple Silicon report against the draft assets. |
| Nonfunctional contribution help entry | Already disclosed; operator guide remains usable. A broader CLI-navigation cleanup is not required to package these targeted fixes. |

No open provider PRs remained at the initial inventory after merging #32/#34.
Recheck proposals/PRs/issues before finalizing the draft and again before any
publication. Record new relevant fixes explicitly; unrelated features do not
automatically block a candidate. Qualification remains bounded native package
smoke plus relevant regressions, with deep corpora/soak left to explicit native
CI. No hundreds-of-thousands-case hosted campaign is requested.

## Known release limits

The exact new Mac binaries still require Pink Menace execution. Source/CI passes
and beta.2 runtime results do not qualify them. Mac files remain without Developer
ID signing/notarization, and the browser/quarantine path remains uncertified.
Native Linux observations must identify their actual CPU and these exact bytes.
The draft must not be published or its source tag created by this packaging step.
