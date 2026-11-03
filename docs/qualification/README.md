# Qualification records

For the current published package status, use
[0.1.0-beta.2 release evidence](../releases/0.1.0-beta.2.md). It records bounded
native smoke for Linux AMD64/ARM64 and Apple Silicon, unsupported Intel Macs,
and pending Developer ID signing/notarization. The
[prospective support contract](../beta-support.md) records broader beta gates.

The files under `optional-extensions/` are **frozen source-bound mission records**,
not a live release dashboard. Their original candidate versions, digests, review
handoff/pending wording, failed or timed-out attempts and exact test scopes are
retained. The extension mission later merged in
[PR #25](https://github.com/LynnColeArt/hubnot/pull/25). Its original runtime
observations cannot be transferred to beta.2 by changing a label.

- `baseline/` records imported prerequisites and historical/default baselines.
- `default/` records the default-workflow matrix and fixed-host measurements.
- `packages/` records earlier candidate packages, runtime rows, CI scope changes
  and explicit exclusions.
- `independent/` records separate AI-agent native-user/extension-author trials,
  boundary review and the readiness handoff at their observed revisions.

Likewise, [the beta.1 platform campaign](../platform-qualification.md) retains
historical source/package identities and failures. Earlier alpha release entries,
`testdata/compatibility/`, and the operational self-hosting record are historical
evidence, not claims that those old binaries are the current download.

Documentation updates must not rewrite old digests, move failures to PASS, or
reproduce a different executable under an old result. Add a new dated observation
bound to its actual bytes. A new package pass does not establish a new source
race-suite pass, universal platform support or full beta acceptance.
