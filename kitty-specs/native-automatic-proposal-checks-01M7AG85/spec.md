# Native automatic proposal checks

Hubnot schedules verified proposal opens and new revisions for opted-in repositories. Workbox consumes the native contract.

## Functional Requirements

| ID | Requirement |
| --- | --- |
| FR-001 | Validate trigger drafts and preserve exact candidate, configuration and pipeline identities. |
| FR-002 | Observe verified revisions and persist scheduling and execution attempts across restarts. |
| FR-003 | Execute with the pinned active target definition and isolated runner, and accept signed evidence only from the canonical target history. |
