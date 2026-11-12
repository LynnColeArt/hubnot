# Safe native proposal publication

Workbox Settings needs to publish an exact, signed Hubnot proposal and recover its result after a lost response. The existing human-oriented proposal command remains compatible.

| ID | Requirement |
| --- | --- |
| FR-001 | Advertise versioned publication and lookup. Accept an exact base, expected base ref, exact candidate head, expected active actor, stable operation key, title, and body. Validate the request under existing Hubnot candidate and contribution policy. |
| FR-002 | Publish at most one signed proposal for the same actor and operation in a repository. Reject a different intent under that key. Verify the base ref and expected actor before publishing a new operation. |
| FR-003 | Return the original signed proposal, exact candidate, and publication readiness on retry or lookup. A retry after branch advancement returns the original result. Repair only the original missing code ref or report a controlled recovery state. |
| FR-004 | Return structured absence, denial, stale input, conflict, and uncertain outcomes. Keep existing proposal open syntax, verification, and proposal read behavior compatible. |

Acceptance uses real Git and Hubnot to verify concurrent duplicate requests, wrong signer, stale base, changed intent, replay after base advancement, and repair after a partial publication without creating a second signed event.
