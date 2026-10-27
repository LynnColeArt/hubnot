# WP05 approval evidence

Reviewed source247a4e1b26295459ae159674c109bcd91aa1a954.
Adjacent review-approved-247a4e1b.md records the verdict.

From lane-d at this source, run a negative probe with:
    go test -race -count=1 -overlay '/home/lynn/projects/nichthub/kitty-specs/portable-collaboration-and-independent-audits-01M5VTP0/tasks/WP05-migration-engine-local-archive/approval-247a4e1b/old-production-overlay.json' -run '^TestMigrationDestinationStaleWorktreeRegistration$' -v .
Replace old-production with hook-fault or missing-object for the other probes.
Each must exit1 with the assertion shown in its log. The ordinary focused run
passes. These are source overlays, not edits to product files. To use another
checkout, adjust overlay JSON paths. implementation/raw.tar.gz is Kat's verified
20-member source-bound bundle. Outer SHA256SUMS also binds the preserved archive.
