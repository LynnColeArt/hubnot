# Selected import validation without unpublished restrictions

Root architectural adjudication, 2026-10-31 UTC. Existing ownership, contracts,
lane identities and default workflow are unchanged.

The exchange object count is capped at 4096; it is not a ref-count cap. Distinct
well-formed remote aliases may point at the same small actor prefix. Ref selection
uses the existing 100000-ref read ceiling together with the published wire and
other exchange bounds. A valid 4097-ref alias case must not fail solely because
the decoder reused the object-count constant. Identical derived destination
prefix targets must deduplicate safely rather than issue duplicate updates.

Export remains minimal and public-only. Accepted import requires a strict bounded
wrapper, valid closed object types, recomputed object-format-specific hashes for
EVERY supplied object, and complete shared-reader validation of the SELECTED
refs and their required closure. The frozen contract does not additionally say
that every supplied object must be reachable from those refs. Do not invent that
rejection condition.

Charge all table entries and bytes to the wire, decoded and object limits,
including unselected entries. After validation, copy only the verified selected
closure inventory and promote only its verified selected refs. Unreachable
entries are neither copied into the destination ODB nor implicitly preserved in
the opaque index. They do not become accepted records, extra roots or a source of
missing-schema authority. Invalid types, hashes, duplicate OIDs/ref names,
missing selected dependencies and malformed selected histories still refuse.
The explicit opaque-preservation path retains its separate all-input-objects
intent and never promotes accepted refs.

The 2048 signed-record bound counts verified selected `hn-artifact/1` records,
including their selected type/prefix dependencies, rather than every raw object
whose Git type is `commit`. Arbitrary opaque commits and unreachable table
commits do not acquire a verified signed identity by that type label. All table
objects and bytes remain subject to the early aggregate bounds; the verified
selected count is checked through the shared history/closure before accepted
copy or promotion. Opaque mode reports zero signed records and may preserve up
to 4096 raw commit objects when the other index/exchange bounds also permit it.
Qualification retains a causal old-binary overconstraint refusal and the actual
opaque CLI positive, as well as the isolated exact/one-over signed-count checks.

WP10 must preserve meaningful failing controls for these draft overconstraints,
then demonstrate real accepted import with a valid unselected public blob while
that blob remains absent from the destination ODB and opaque index. An invalid
hash on an unselected entry must still refuse before effects. Independent review
must check those facts and the precise published bounds, rather than treating a
stricter draft implementation as the specification.
