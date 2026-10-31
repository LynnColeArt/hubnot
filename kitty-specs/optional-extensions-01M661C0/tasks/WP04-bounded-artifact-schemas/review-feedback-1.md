# WP04 integration rework: separate value validation from CLI consumption

The previously approved source `e885669056611613360e092d08b572244b31013e` correctly implements the inert CLI's 16 MiB composite-input cap. WP05 integration exposed a separate requirement: selected accepted histories may legally supply a larger combined parent context under the existing 256 MiB history budget. Reusing `ValidateRecord` by serializing that context wrongly imposes the utility's input cap on storage reads.

WP05's retained `TestArtifactStoreLargeVerifiedParentContextCLI` constructs a valid signed accepted history of approximately 19 MiB with individually bounded envelopes, then calls the actual `hn artifact type show`. It fails with artifact/resource_limit, `validation input exceeds byte limit`. Evidence: external `optional-extensions-WP05/large-context-red.log` and `large-context-red-source.json`. This is integration rework; it does not retroactively claim that the old inert CLI approval covered a storage context API.

Required correction within WP04's existing owned `internal/artifact/content.go` and tests:

- Extract the existing record semantic validation into one private value/context core. The live `ValidateRecord(ctx, raw)` must retain strict parsing and the unchanged 16 MiB input cap, then call that same core.
- Preserve all existing signature, exact-schema, immediate-parent/opening, conversion, closed-field, cancellation and error semantics. Do not copy the validator, relax a limit, change signed bytes, or add a second implementation.
- Allow WP05's already-owned `internal/artifact/canonical_export.go` to provide a minimal current-reader wrapper over the private core. Its input comes from the bounded, independently verified selected-history loader; per-envelope, parent-count, modeled-depth and validation-time bounds still apply. No giant composite serialization or new unlimited input surface.
- Add meaningful core/context tests and retain the current live CLI positive/refusal controls, including the unchanged over-limit utility refusal. The actual larger-history public read must turn green after WP05 integrates the freshly approved dependency; keep that later evidence obligation explicit.

Root requests normal canonical rework and a new independent verdict for the changed dependency. Existing approval history stays intact. WP05 remains paused at this integration boundary until the revised WP04 source is independently approved and merged into its lane. No ownership transfer, force/skip, default-workflow change, native protocol change or release action is authorized by this rework.
