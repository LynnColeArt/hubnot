---
affected_files: []
cycle_number: 3
mission_slug: optional-extensions-01M661C0
reproduction_command:
reviewed_at: '2026-10-31T03:10:16Z'
reviewer_agent: codex
wp_id: WP05
---

# Shared public reader source boundary

WP08 inspection of the approved reader found that artifactObjectReader.commit already supports an explicit Git directory and context, while loadArtifactHistory combines current-repository discovery, source enumeration and chain resolution. Quarantined Git/file transfer must reuse that same verifier on an explicitly selected public source set. Copying the resolver or changing global working-directory/environment state would create a second validation path.

Root requests a narrow normal integration rework in WP05's existing artifact_read.go and artifact_read_test.go ownership:

- Extract a private loadArtifactHistoryFromSources(ctx, dir, sources) core over bounded, explicit public sources. The existing loadArtifactHistory wrapper keeps its ordinary repository discovery, selected-source semantics and complete source-set stability checks, and calls this same core immediately.
- Preserve the existing object hashes/types/tree modes, signatures, actor prefixes/forks, record/type/schema/required-target checks, cancellation and read/catalog budgets. Explicit sources must be validated, not silently broadened or replaced by current working-directory/configuration discovery. Recheck the selected ref/OID witnesses against the explicit directory; ordinary wrapper discovery still checks additions/removals in its selected set.
- Use an actual separate public bare/quarantine fixture while the caller remains in another repository. Verify identical signed identities, precise failure on changed/missing/invalid selected objects or witnesses, and no signer/private-state creation. Do not copy a resolver, add a second writer, change global cwd/env, relax a bound or change wire semantics.

The new private core has an immediate ordinary-reader caller; WP08's live closure inspector consumes it and WP09/WP10 later use it before accepted promotion. This is not an unused transport SDK. WP08 retains its four owned files; it must integrate the freshly approved dependency before final qualification. Existing tested-source/approval history remains preserved, and the changed dependency receives a new independent verdict. No ownership transfer, guard bypass, release or main-branch action follows from this request.
