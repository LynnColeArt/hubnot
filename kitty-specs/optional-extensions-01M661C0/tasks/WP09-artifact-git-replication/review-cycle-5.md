---
affected_files: []
cycle_number: 5
mission_slug: optional-extensions-01M661C0
reproduction_command:
reviewed_at: '2026-10-31T05:55:16Z'
reviewer_agent: codex
wp_id: WP09
---

# Local fetch acknowledgement classification rework

Root integration rework, 2026-10-31 UTC. WP11's new public-only acceptance
regression `TestArtifactCombinedFetchLostAcknowledgementCLI` is committed at
`293f617feb6c418968b57d5c9c17ee2fba98ad03` in lane-j. Exact evidence lives under
`optional-extensions-WP11/transport-refresh/local-fetch-red/`.

Against the actual refreshed standard executable from source `6cb12307`, the
destination accepted transaction committed the proposed frozen public successor
and then its wrapper returned a nonzero exit status. Exact accepted-ref witnesses
and a committed marker prove publication; the unrelated public sentinel stayed
unchanged. The CLI returned `artifact/concurrent_update` with `retry_identical`.
The frozen post-dispatch lost-result contract instead requires
`uncertain_operation` with `inspect_operation`. A genuine pre-commit stale-old
atomic control passed in the same run. No earlier-owner source was edited.

WP09 must correct the blanket error mapping around local fetch's
`promoteReplicationRefs` call in `artifact_replication.go`. Joined process failure
is not proof of non-publication. Inspect bounded selected target witnesses where
needed, preserve uncertainty when they show possible commitment or cannot settle
the outcome, and retain the meaningful known pre-publication CAS refusal. Never
invent an actor/operation key for this transport or automatically change/retry the
selected public intent. Preserve the newly approved named-ref isolation control.

Use existing owned files only and a small current-call implementation, not a new
retry framework, second writer, parser copy or unused API. The integration test
remains WP11-owned; WP09 may add an appropriate producer-level regression using
its existing public fixtures. Both supplied variants must demonstrate committed
but unacknowledged fetch, genuine stale-old atomic refusal and unavailable
witness uncertainty, with exact source/ref/public-state evidence.

No private signing-head recovery, signing-key manipulation, old blocked auxiliary
probe, remote-server policy or unrelated native recovery experiment is in scope.
The demonstrated case uses only public data in temporary repositories. Run
focused current checks, preserve exact prior unchanged evidence, and complete the
normal subtask/gate/independent-review cycle. This is a new integration correction;
earlier exact-source verdicts remain historical evidence. WP11 stays active and
will merge the new approved dependency before completing its refreshed review.
