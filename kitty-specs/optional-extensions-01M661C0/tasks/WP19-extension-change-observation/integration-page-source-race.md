# Existing-page source races require rescan

2026-10-31 — root, architect-alphonso.

Frozen `contracts/host-v1.md` section 5 requires every retained page to recheck its accepted source/view and to invalidate when that coherent capture can no longer be verified. This includes a source change detected inside a shared native/artifact reader during the page recheck, before WP19 can compare the assembled cut.

For an existing `changes.page`, a verified source-race `concurrent_update` from that recheck must therefore forget the capture and return `checkpoint_invalidated` with `rescan_required`. Do not leave an apparently usable cursor after this failed verification. Initial `changes.capture` and `changes.since` assembling a new replacement retain their inherited source-race error/recovery semantics. Preserve distinct verification/history, authorization, unsupported, cancellation and other failure causes; this is not a blanket error conversion.

WP19 owns this adapter-level correction and focused source-race controls. Existing-page invalidation and unchanged initial-capture behavior need concrete causal evidence. This interpretation adds no public error, contract limit, authority, file owner, dependency, or default-native action. Preserve exact tested source/binary identities when reusing unaffected temporal evidence; no old run may be relabeled as a newer source.
