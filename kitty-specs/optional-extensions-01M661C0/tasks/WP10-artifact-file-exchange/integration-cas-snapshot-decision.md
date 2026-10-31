# CAS refusal snapshots and empty Git bookkeeping directories

Root architectural/test adjudication, 2026-10-31 UTC.

The deterministic two-ref stale-target control exposed Git prepare creating a
previously absent selected-ref parent directory before refusing another old-ref
comparison. No selected ref committed and no file content or private publication
state changed. The original filesystem snapshot treated this empty directory as
a failed atomic-publication guarantee.

Artifact contract section 5 and WP10 T042 require an atomic selected accepted-ref
transaction with no partial promoted heads. They do not promise that Git creates
no empty ref-parent directories while preparing a transaction. Production cleanup
of such directories would introduce avoidable interference with concurrent Git
writers solely to satisfy a stronger test assumption. Do not add that cleanup.

Preserve the original failure and deterministic actor ordering. Narrow only this
joined, witnessed pre-commit CAS-refusal assertion: enumerate the ancestors of the
actual selected ref names, require every newly observed filesystem node to be a
directory in that exact set, and compare the complete file/symlink/content and
private-state inventory unchanged. New directory chains may contain only those
new empty descendant directories, never files or links. Independently assert the
exact stale-target error and that every selected ref remains old or absent.

This is not a blanket directory exclusion, a changed expected Git error, or an
allowance for arbitrary files, objects, private descriptors or partial refs. Other
before-copy refusal snapshots remain exact. Possible committed/post-commit
outcomes retain the distinct uncertainty checks. If anything beyond this precise
empty-directory bookkeeping changes, retain it as an implementation finding.
Independent WP10 review must examine the failure and corrected assertion.
