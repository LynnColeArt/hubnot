# Custom host Cut scope, including empty and opaque projections

Root architectural decision, 2026-10-31, informed by independent Reviewer Renata
advisory wp18-cut-advisory/advisory.md and literal vectors. This explicitly resolves
underspecified internal selection/ID-set semantics; it is not presented as wording
already mechanically required by the frozen text. Wire fields, hash formula,
grants, inventories, limits and the nonempty public changes selection stay fixed.

Correction: approved artifactHistory.sourceCut already uses the frozen six-field
outer object. Root's earlier different-outer-preimage statement was inaccurate.
That core fingerprint selects every loaded type/ByID. A narrower host selection
must bind its own verified effective type projection instead of blindly borrowing
the whole-history scope. Reuse the shared verified reader, catalog and diagnostics;
do not introduce another writer, schema/lineage verifier or activation mechanism.

Use the frozen formulas, canonical encoding and full D digests:

    Cut = D(C({repository_id,refs,selection,authority_commit,policy_digest,view_digest}))
    view_digest = D(C({admitted_ids,visible_ids,retained_refs}))

Normal typed host methods use a complete effective-type projection T. Explicit
type_id remains in T even if empty; an exact ID resolves its actual verified type
and must agree. Broad queries expand actual catalog types under applicable grants,
never wildcard strings. Sort/dedupe T. Bind exact method/query/id/mode/shape/search,
registration/epoch/grants and limit-independent offset separately in continuation
logic. This chooses one consistent type-wide convention rather than varying the
Cut ID-set convention by endpoint or page.

Use the physical verified host repository binding. refs contains ALL actual
accepted h.Sources used by the coherent read, sorted/deduped by ref, even streams
currently without an item in T; shared streams and operation keys span types.
Never substitute derived export-prefix refs. Custom authority_commit/policy_digest
are empty strings and retained_refs is []; no native governance is invented.
For a successful supported custom projection, admitted_ids and visible_ids are
the sorted exact declaration and record revision IDs in complete T after shared
contextual/catalog/dependency/diagnostic verification. Equality follows that
successful no-custom-policy projection, not raw signature validity. Unsupported
or incomplete diagnostic-only objects are not silently promoted into usable view
IDs. Preserve real diagnostics and errors. Every returned body/head still needs
its own authorization; digest membership grants no disclosure or authority.
After projection, recheck the complete exact accepted ref set, including additions
and removals, and session/binding/grants before returning the coherent result.

The nonempty ReadSelection rule applies to externally supplied changes.capture/
changes.since selections and corresponding ChangePage selection. It does not turn
an empty internal Cut projection into an implicit-all request:

- A permitted namespace search with no observed eligible types has internal
  selection={families:[],types:[]}, empty view IDs, and all actual accepted h.Sources
  retained in refs. It completes only that grant-filtered observed query, never
  proves global absence. An explicitly supplied exact type remains in T instead.
- Exact one-object opaque inspection has that empty internal typed selection,
  refs=[], empty authority/policy strings, and the real D digest of the canonical
  empty admitted/visible/retained object. It claims no accepted artifact membership,
  governed visibility, global absence or changes checkpoint. Do not invent a
  type, signed ID, Git ref or authority for raw quarantine data.

Opaque lookup still verifies exact immutable opaque_id, current public-index
membership/hash/bounds and raw.inspect binding/epoch/grant on each request. Success
returns one complete opaque item with next_cursor=null; missing is not_found.
No continuation is issued for this single-object selection; a supplied continuation
that cannot name an issued page refuses checkpoint_invalidated. Oversize remains
resource_limit with diagnostic-export guidance. There is no arbitrary ODB/path or
private-state fallback. Different opaque IDs in one repository intentionally share
this empty-projection Cut: the exact opaque_id is the content/query binding. Never
use that Cut alone as an object cache key, permission or quarantine-removal bypass.
This does not authorize a resumable opaque collection or an empty changes request.

Independent synthetic metadata vectors (not signed histories or runtime passes):
normal view 639debba1a9e75c6ba34529cb3fce1651c27e3a375a39f9e1566fe6facd9df2e;
normal Cut 4e658ed082cea5c9be2439fc4528e19051b89abe62d9d7fcf2af725b4d6aaf1a;
empty view 1f3340ea256c3808d709ad97cb10dc1f379f9ddd6a7277c4b6bc43221a200f9d;
empty namespace Cut 22e9db95091b80ddadd2a6ad555e2b52477ef2674d1be61c2204e8f515c601cb;
opaque Cut df2e63a287b224bc0adfb69fd27badcb35bfced67afefaeb8c623177f6233618.
Each is prefixed sha256: in actual data. Exact no-newline preimages and independent
sha256sum checks remain in external wp18-cut-advisory/vectors.json and companions.
WP18 review must independently check mapping, fingerprints, actual read/opaque
boundaries and no-effect refusals. WP19 must reuse the same verified projection
semantics for explicit accepted replacement captures; this decision grants no
custom governed feed under selected contribution policy.
