# Concept boundaries

## Portable project snapshot

An immutable selected observation of a project: source identity/revision,
selected code refs/history, exact native facts, attributed foreign records,
relationships, retrieved attachment digests and explicit coverage findings.
Its integrity manifest is not a grant of native authority.

Native histories are admitted using native signature/dependency/quarantine
rules. Foreign records retain provider instance + immutable project/repository
ID + object kind + immutable object ID separately from mutable names/selectors.
Source authors/times/reviews remain descriptive provenance. Attachment blobs are
content-addressed, bounded and verified; a URL alone is not a retrieved blob.

## Migration plan and progress

A plan binds the source observation, explicit destination, scope, capability
findings, mappings/loss decisions and intended changes. It contains no secrets.
Progress binds each intended write to prepared, in-flight and confirmed states;
an uncertain result needs positive reconciliation or manual recovery.
The original snapshot cannot silently refresh during resume.

## Provider capability

A resource + direction + source/version/endpoint contract describes mapping,
preservation and unsupported cases. Issues, repositories, boards, work items,
standalone discussions and attached threads remain distinct concepts.
Adapters translate through shared contracts; native storage and orchestration
do not assume provider-specific permissions, types or CI languages.

## Independent audits

An audit request identifies resolved target code, authorized rule source/revision,
requester and explicit execution intent. A result identifies its request,
executor, outcome, bounded evidence/logs and stable findings. These facts form
their own signed family and never satisfy merge review/CI/acceptance evidence.

A finding identifies an observed problem independently enough to support
intentional follow-up and rerun deduplication. A follow-up link binds a finding
to a native issue and the actor-scoped retry identity. Concurrent publishers
retain visible provenance/conflicts, not an invented distributed lock.

## Release artifacts

Artifacts bind an exact source revision, supported compiler, target, variant,
archive paths/modes and digests. Packaging is a local Go/Git operation;
publication, signing/notarization and runtime qualification are separate actions.
