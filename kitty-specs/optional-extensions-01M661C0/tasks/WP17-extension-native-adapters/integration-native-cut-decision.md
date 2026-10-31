# Native source witnesses and governed view cuts

Root architectural clarification, 2026-10-31, against frozen host-v1 sections 3–5.
This changes no Cut schema, wire inventory, authority or grant.

The existing native reader selects complete shared actor streams: local
`refs/hn/actors/` and remote observations accepted by `parseAcceptedActorRef`.
Actor continuity, cross-family native operation keys and governed projection
validation use that complete verified selection. Use those actual accepted ref/OID
witnesses as native `refs`; do not minimize them to streams that already contain
an event of the requested family. Such minimization can omit a witness used to
prove cross-family key absence. Conservative invalidation when another native
event advances a shared stream is acceptable. Returned resources, admitted IDs
and visible IDs remain restricted to the requested family/selection and grants.
Internal digest inputs do not grant disclosure of other-family bodies or IDs.

`authority_commit` and `policy_digest` are the exact selected authority values,
or empty strings as specified. `view_digest` uses the canonical object and sorted,
distinct selected admitted/visible identities and retained ref/OID observations.
Verified selected-project contribution manifest heads may be retained witnesses
when the same successful governed projection authenticates them and verifies
their registered retention coverage. Keep them in `retained_refs`, not native
accepted `refs`. ReadContributionManifest explicitly calls its records
signature-authenticated retained data, not admitted authority: preserve that
distinction. Membership in either digest never admits, activates or exposes data.

Observe the exact same ref/OID, authority, policy and retention inputs before and
after the coherent read, and refuse a changed/incomplete observation. No guessed
ref name/OID, unchecked extra prefix, working-tree policy replacement or raw reader
snapshot string may substitute for the frozen Cut object. Keep existing finite
read limits and synchronous request ownership. Future change capture must reuse
the same current native projection/fingerprint semantics rather than invent a
parallel feed or timestamp ordering.

WP19 preparation confirmed the current projection retained only the final Cut
hash while its verified arrays were local. Retain a typed Cut-input value in that
same projection and make the EXISTING current single-family hash assembly consume
it. This is reuse of actual accepted witnesses/selection/authority/view inputs,
not a new multi-family API or unused parallel projection. Preserve exact current
Cut bytes and prove them independently. A later capture can then combine coherent
verified inputs rather than reverse-engineering a digest or copying this reducer.

Independent review must recompute the fingerprint inputs and test both a normal
selected change and the cross-family shared-stream/key case. This is clarification
of current native source selection, not permission to export ungranted history.
