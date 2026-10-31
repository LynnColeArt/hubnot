# Typed host selection and capability interpretation

Root architectural clarification, 2026-10-31, from frozen artifact-v1 section 3
and host-v1 section 4. No wire schema, inventory, grant or ownership is changed.

Artifact conversion keeps the logical opening, type_id and shape; it explicitly
changes the exact type_revision. Cross-namespace conversion is invalid_input,
not an unimplemented feature to add. Retain source/destination grant checks and
the shared successor validator; neither an invalid conversion nor a denied grant
may reach signer, private intent or publication.

Host artifact.read does not export the operator CLI's actor-prefix closure:

- view/heads use the complete selected RecordView values from the same verified
  current catalog.
- history with id selects the complete logical record revision history, using
  the existing catalog's deterministic order and paging. It does not append
  unrelated actor-prefix entries or dependency bodies.
- inspect with id selects that exact signed artifact. A supplied type_id must
  agree. Inspect with only type_id selects that type's signed artifacts from the
  verified catalog, with complete deterministic paging for that selection.
- type.read selects exact type declarations, under its separate frozen method.
- explicit opaque inspection retains its different opaque_id shape, quarantine
  source and raw.inspect grant; no accepted-object/path fallback is permitted.

Verify signatures, types and required dependencies through the shared reader and
catalog/inspection machinery internally. Preserve actual unsupported/incomplete
diagnostics; do not call invalid history an empty result. Internal verification
does not authorize exporting a dependency or an unrelated actor-prefix body.
Authorize every returned item's actual type/payload, including each head. Keep
typed query and cursor selection exact, all full-state/frame limits, and honest
completeness of the selected set. A grant-filtered selection does not prove global
absence. If a selected complete value cannot be returned under the grants, refuse
rather than silently deleting its unauthorized history and claiming completeness.

The frozen capability inventory contains all fourteen rows. `availability` equal
to observed means the successful session handshake, as host-v1 explicitly states;
it does not claim every possible method/type/policy combination was executed.
Declared/granted/enabled/compatible retain their specified distinct meanings and
each actual request is checked. No new SessionConfig/runtime availability seam is
required for WP18. Its advertised artifact methods require live positive/refusal
evidence; WP17 and WP19 provide the other current callers at final integration.
The final mission must demonstrate all fourteen methods and thirteen explicitly
unsupported privileged surfaces. This clarification does not excuse a missing
final caller or permit an unavailable method to return fabricated success.
