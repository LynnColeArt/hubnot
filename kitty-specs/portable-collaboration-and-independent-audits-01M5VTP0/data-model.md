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

Immutable intents may contain typed references to confirmed predecessor receipts.
The engine resolves and persists canonical actual payload bytes/digest before
the single side effect. Both apply and reconciliation receive that same bound
write, immutable receipt values and verified bounded blob access; adapters keep
no hidden parent-ID or payload-rewrite state.

## Provider capability

A resource + direction + source/version/endpoint contract describes mapping,
preservation and unsupported cases. Issues, repositories, boards, work items,
standalone discussions and attached threads remain distinct concepts.
Adapters translate through shared contracts; native storage and orchestration
do not assume provider-specific permissions, types or CI languages.

## Independent audits

An optional committed audit policy declares scope and separate rule-owner,
requester and executor actors. A separately adopted private local anchor selects
the current named authority branch with expected-old updates; it is never
imported or inferred from a candidate/checkout. Policy-ref arguments only assert
the current selection. Policy byte changes and role/rule-head changes are checked
before execution; code-only advances with unchanged policy need not invalidate a
request by commit alone. Owner-signed rule authorizations/revocations form explicit
successor graphs; sole owners may authorize their own rules. Concurrent heads,
current revocation or stale authority block new execution without erasing
historically valid authorizations/results. Sources initially remain inert/local
or explicitly already-present external Git data; no implicit fetch is introduced.

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
