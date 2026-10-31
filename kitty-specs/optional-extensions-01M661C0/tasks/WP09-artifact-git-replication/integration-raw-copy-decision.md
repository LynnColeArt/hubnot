# Preserve verified raw Git bodies during artifact transport

Root architectural adjudication, 2026-10-31 UTC; resolved architect profile and
design charter context apply. This is a contract-fidelity correction within the
existing WP09/WP10 file ownership, not a signed-contract amendment.

Independent WP09 review demonstrated that the frozen accepted type-note and
optional-missing-target vectors can be read successfully by the shared accepted
reader, yet both supplied transport binaries refuse to copy their exact Git
commit bodies. The cause is Git's `hash-object` author-email check. A comparator
changing only unsigned author/committer metadata to angle-bracket email syntax
passes while keeping every artifact ID, envelope and signature unchanged. The
review preserves both cases; no frozen vector was rewritten.

Artifact contract section 5 specifies exact raw public Git objects and treats
author/committer metadata as transport metadata rather than signed authority.
The accepted fixture and shared reader must not silently acquire a stricter copy
condition. The fix stores the exact bounded, hash-checked raw bodies, using
`hash-object --literally` in the artifact-specific object-copy path where needed.
It must not normalize metadata or create replacement Git bodies to hide the gap.

An import may store untrusted raw bodies only in its application-owned quarantine
before shared signature, chain, tree, schema and closure validation. Accepted
promotion still requires all those checks. Object types remain the closed
blob/tree/commit set; full type/length Git hashes, object-format matching and
resource limits remain mandatory. This decision changes no native copy path,
remote validation configuration, hook policy or authority rule. A remote's own
refusal remains an error; it is not permission to disable that refusal.

WP09 and WP10 each require a meaningful frozen-literal regression through their
actual transport interface and an independently initialized destination. Original
body bytes and OIDs, as well as signed artifact identities, must survive. Their
independent reviewers adjudicate the completed fix before approval.
