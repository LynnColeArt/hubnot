# Native consumer CLI v1 contract

Commands: proposal show PROPOSAL --json; proposal status PROPOSAL --json; log --json; review FULL_PROPOSAL (--approve|--request-changes) [--body TEXT] --json; decide FULL_PROPOSAL (--accept|--reject) [--body TEXT] --json. Add a documented read-only event show/resolve path and static version --json/capability discovery. Exact final command and capability names must be recorded with tests/docs by WP01/WP03.

Envelope is {ok,data,error}, exactly one meaningful data/error branch. Explicit schema/version in data/discovery makes capability detection reliable. IDs are full sha256 identities; base/head Git object IDs remain full native object IDs. Error is exactly {origin,code,message,recovery,operation} using docs/extensions.md native inventory; operation:null for these unkeyed commands, sanitized message, exit 1; success exit 0. No automatic retry or idempotency recovery promise.

Proposal data: candidate full ID/kind/author/display name/base/head/policy; current_reviews object keyed full actor fingerprint with full review event ID, verdict, sequence and optional display name; requirements including reviewer pool/required count/actual/deficit, protected checks, pipeline results and evidence; lineage predecessor/successors/current/closed/superseded; separate readiness, disagreements, accepted/rejected/merged observations, acceptance/merge eligibility. Reuse native evaluation, do not introduce assignments/unanimity. Filter operative evidence before current review selection. Empty collections have stable JSON shapes. Document any extra dependency refusal JSON show requires.

Review receipt: exact created full event ID and its signed subject/verdict/actor. Decision receipt: exact created full event ID, subject/verdict/actor, stored Policy digest and stored Evidence list. Never recompute receipt from current status. Evidence roundtrip means compare status evidence references for the proposal ID and read/resolve each evidence event separately.

Read-only event resolution uses existing store.go resolveEvent; ambiguous prefixes refuse native ambiguous_id. Do not relax requireFullEventID for any mutation or artifact. Reads preserve existing verification, moderation, pending/shallow history selection boundaries; append no facts or signer migration. Log boundary is explicitly the existing raw history boundary.

Static discovery works keyless outside repository without contribution lock, Git IO, authority or implicit initialization. Native-only builds advertise only compiled support. Nine contribution json flags retain actual always-JSON default output bytes, documented as existing separate schemas; no envelope migration in this mission.

All existing non-JSON output and exits byte-identical. No event schema/signature/authority changes. Validation must prove actual CLI behavior, not just helper serialization.
