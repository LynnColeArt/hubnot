# Opaque public preservation ownership and bounds

Decision recorded by root with the resolved `architect-alphonso` profile and
design-scoped charter context, 2026-10-31 UTC.

The frozen artifact contract sections 5 and 7 already require explicit opaque
preservation, inspection and diagnostic export. WP10 implements those file
exchange behaviors inside its four existing owned files. WP09's ownership of
`quarantine.go` covers the existing Git transport plumbing; it does not require a
second opaque index. Normal Git fetch/push has no opaque-preservation flag and
must not implicitly retain unsupported material as addressable opaque data.

WP10 stores explicitly preserved public object bodies in the physical private
Git-directory location `hn/artifact-opaque/public.json`. This is a strict
`hn-artifact-exchange/1` envelope with mode `opaque` and an empty refs array.
The aggregate index, not merely each incoming request, is bounded to 16 MiB wire,
32 MiB decoded object bodies and 4096 objects. Objects deduplicate by their exact
Git OID. Equality is permitted when every other condition is valid; an over-limit
merge refuses before replacing the prior index. The wire bound can bind before
the decoded bound because Base64 expands the input; qualification must report the
actual boundary reached rather than invent a 32 MiB decoded wire success.

The implementation must use the existing physical-directory and regular-file,
no-symlink safety patterns, serialize read/merge/write operations, and replace a
complete validated index atomically. Concurrent writers must not silently lose
preserved objects. Any refusal retains the prior index. This is an implementation
policy within the published exchange bounds, not a new signed or public wire
format, registration requirement, or accepted ref.

The only opaque lookup source is that bounded public index. Identity is exactly
the contract digest of UTF-8 Git object type, NUL, and raw object body. No arbitrary
Git ODB, private path, signing key, journal or credential fallback is permitted.
Strict wrapper validation and correct Git object hashes remain mandatory even
when the contents cannot be interpreted as a supported artifact. Opaque data
never becomes a signed artifact identity, accepted revision, policy or authority.

WP10 supplies the current CLI consumers for explicit preserve/import and opaque
inspect/export. Its own command file may wrap `inspect` and delegate ordinary
inspection directly to the approved closure handler, without editing WP08's
files or relying on captured map-initialization order. WP18 later consumes the
same index reader under its explicit opaque grant; no dormant host method is
introduced in WP10. Native/default commands perform no index I/O.

`import --preserve-opaque` explicitly selects preservation only for all supplied
hash-valid public objects, whether the input wrapper says `accepted` or `opaque`.
The result is mode `opaque` with no accepted promotion. It is not an automatic
acceptance attempt followed by a hidden fallback. An opaque-mode input without
the explicit flag refuses with the frozen `invalid_input` code. Strict wrapper,
Git hash and resource checks still apply; malformed JSON never enters the index.

Normal accepted import and Git fetch/push reuse `artifactMutationPolicy` before
transport, object-copy or public-promotion effects. The selected-contribution
combination remains `host/unsupported_capability`: a transport must not bypass
the existing prohibition on custom publication under that policy. Public import
does not require a local signing identity. Read-only export and explicit opaque
preservation do not admit accepted custom data or advertise a governed custom
feed. Existing native command-lock bookkeeping remains intact.

This decision changes no work-package ownership, lane identity, signed contract,
default workflow or mission scope. Independent WP10 review must verify these
concrete storage and CLI behaviors before approval.
