# File import named accepted-ref isolation rework

Root architectural rework, 2026-10-31 UTC. Read the paired WP09
`integration-ref-namespace-rework.md` and its independently produced static
advisory/hash. No runtime reproduction was claimed by that advisory.

WP10's `artifactExchangeSetRefs` must update only the exact named accepted refs,
even if a pre-existing destination is a symbolic ref to an unrelated public
sentinel. Expected-old OID checks and atomicity alone do not prevent default Git
dereferencing from changing that other named ref. Apply the minimal transaction
no-dereference protection in the existing owned file, without modifying earlier
owners, native publication paths, signatures, Git bodies, format or quotas.

Retain a meaningful old-code regression through actual file import in temporary
repositories with a valid public artifact prefix and unrelated public sentinel.
No signing-head recovery, private-key manipulation, old blocked auxiliary probe
or remote server experiment is authorized. Prove the sentinel's exact OID/body
unchanged after the corrected import, plus correct accepted-ref outcome. Existing
stale-old, atomic multi-ref, post-commit lost-acknowledgement, unavailable-witness,
strict-input, closure and opaque-only controls remain meaningful and unchanged.

Use only the existing four owned files, exact source/input and standard/native
binary bindings, focused relevant checks, normal canonical subtask restoration
and the synchronous for-review gate. This is a new scoped integration rework,
not an edit to an earlier verdict. Independent review must approve the correction;
WP11 then refreshes the two approved transport dependencies before downstream
qualification. No ownership, lane or signed-contract amendment is needed.
