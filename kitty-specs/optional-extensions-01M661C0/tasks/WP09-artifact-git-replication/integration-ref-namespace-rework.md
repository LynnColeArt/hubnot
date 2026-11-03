# Named accepted-ref isolation rework

Root architectural rework, 2026-10-31 UTC. Independent read-only source advisory:
`optional-extensions-ref-namespace-audit/advisory.md`, SHA-256
`3b9b19fa0cf9a69a2b153f6c194bec49b4ff85036929bde3d30a1af04c307b19`.
The advisory performed no application, test or probe execution.

The newly introduced artifact fetch/import paths validate an accepted destination
name A and expected OID, but their local Git ref transaction does not explicitly
disable symbolic-ref dereferencing. A pre-existing symbolic A can name unrelated
ref B holding an otherwise valid public artifact prefix. The inspected readers
and expected-old checks accept that prefix; default dereferencing can then update
B. Atomicity and OID CAS are not bypassed, but the documented guarantee of writing
only the accepted namespace is lost. This is a conditional local-repository
state, not a claim of remote-only symbolic-ref injection or remote-server policy
bypass.

WP09 owns the artifact fetch path and `quarantine.go`. Correct the new artifact
publication transaction so it acts only on the exact named accepted refs, including
if a destination is or becomes symbolic. Prefer the minimal transaction-level
no-dereference control; a preflight check alone does not close the update window.
Keep existing expected-old atomicity, closure verification, cancellation and
acknowledgement semantics. Keep the legacy native publication code path unchanged;
an artifact-specific branch in the shared helper may select the direct-ref option
without copying a second writer or adding an unused future API. Any indispensable
broader change needs a new explicit root decision.

The owner must first retain a meaningful ordinary regression against the current
artifact code using an unrelated public sentinel ref and a valid public prefix in
temporary repositories. Compare that sentinel's exact OID/body and all named
accepted refs before/after actual fetch; then prove the correction updates only
the named accepted destination or refuses without unrelated publication. No
signing-head recovery, signing-key manipulation, old blocked auxiliary probe or
legacy native recovery experiment is in scope. Existing signatures and raw Git
bodies must remain unchanged. Retain exact cause, source and binary identities.

Use existing owned files only. Restore subtasks through normal canonical commands,
qualify appropriate focused standard/native cases and native compatibility where
the shared helper is touched, and complete the normal for-review gate. Independent
review must verify the new boundary. Prior reviews remain historical evidence for
their exact earlier source; this normal integration rework does not rewrite them.
WP10 has the matching import correction. WP11 must integrate both newly approved
sources and requalify its combined dependency view before downstream final checks.
