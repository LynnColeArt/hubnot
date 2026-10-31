# Integration correction: complete observation revision identities

2026-10-31 — root, architect-alphonso.

WP19's real issue/comment capture exposed an inherited WP15 response-validation defect. `changePageRule` reuses `idsRule`, whose 1024-member bound belongs to artifact parent/conversion arrays. The frozen host contract section 5 instead requires `ChangeEntry.revision_ids` to include every visible current head and, for issues, every visible comment ID. A legal response with 1025 revision IDs and only 262010 canonical entry bytes was refused by the typed response validator; 1024 IDs passed. The original runtime evidence is retained externally at `optional-extensions-WP19/revision-array-seam-red.log`.

Reopen WP15 through the canonical rework path. Its owner must change only the `changePageRule` revision-ID array to the existing frame-bounded sorted/distinct ID sequence style, with focused protocol positive/negative controls. Do not relax `idsRule`, parent/conversion/schema bounds, ordering, uniqueness, ID syntax, the 8 MiB wire limit, or any request shape. This corrects implementation fidelity to the existing contract; it does not establish a new public limit or change native signed data/default behavior.

WP19 owns the actual signed issue/comment producer and raw `changes.capture` regression at 1024/1025 revision identities. Keep its original failure and source identities. After independent WP15 approval, merge that exact approved dependency into the current WP19 lane, preserving the approved native/artifact source union and WP19-owned work. WP16–18 have no authored change: the corrected response rule applies to the change methods first implemented by WP19. Their historical approvals remain evidence for their exact source; final integrated qualification still belongs to WP24/WP25.

No file ownership, lane identity, dependency graph, frozen contract, beta acceptance, main branch, release, or authority expansion is authorized by this correction. Root continues to serialize application execution; the WP15 author may inspect/edit while WP19 holds the runtime slot.
