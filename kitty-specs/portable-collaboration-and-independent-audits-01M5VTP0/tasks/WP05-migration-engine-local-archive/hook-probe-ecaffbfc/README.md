# WP05 independent hook-boundary probe

Fixed source: `ecaffbfc89affc63bf6af7289a55cd3ac403bdff`; lane-d. No product edits.

`migration-hook-probe.go.txt` is an overlay-only Go regression through actual
`ExportProjectSnapshot` → `PrepareMigration` → `ApplyMigration`. It configures a
relative destination `core.hooksPath`, places a successful source/caller hook and
a rejecting destination hook at that path, then applies a selected code snapshot.
Both bare and ordinary worktree destinations incorrectly return success, execute
the SOURCE hook twice, and publish the observation ref. An independently anchored
`git -C <destination> --git-dir=<destination-git-dir> update-ref` control correctly
executes the DESTINATION hook and rejects the write. Marker bytes and output are
preserved here; temporary repositories were ordinary Go test temporary data.

Reproduce from the product worktree by running `prepare_overlay.py WORKTREE SCRATCH_DIRECTORY`
and executing its printed command there. Expected failing exit at reviewed head: 1.
The script creates scratch files only. Linked-worktree behavior is part of the
required correction/regression scope and is not claimed exercised by this probe.

`existing-migration-tests.log` is an independent `go test -race -run '^TestMigration'
-count=1 -v .` pass (7.910 s). `internal-migration-tests.log` is the independent
`go test -race ./internal/migration -count=1 -v` pass (1.057 s). The fixed-head full
race/vet/build/format/diff evidence in adjacent `validation-ecaffbfc` was reused;
its complete `SHA256SUMS` verification passed. No duplicate full suite ran.

Scope and hashes are in `source-binding.json`; `SHA256SUMS` covers every artifact.
