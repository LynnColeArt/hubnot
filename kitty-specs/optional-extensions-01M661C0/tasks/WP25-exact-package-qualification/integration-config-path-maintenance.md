# Final qualification owns bounded test-detector maintenance

2026-10-31 — root, architect-alphonso. Prospective maintenance scope, not a runtime pass or acceptance waiver.

The platform preflight is recorded at `/home/lynn/Documents/Codex/2026-10-24/wa/work/optional-extensions-final-qualification-prep/harness-and-config-path-audit-20261031.md`. Linux config-path assumptions can miss the actual Darwin registry or cause ENOENT. A private-key check also refers to an obsolete path. Correct those tests permanently before final package qualification.

## Exclusive transfer

- WP03 → WP25: `artifact_envelope_commands_test.go`.
- WP04 → WP25: `artifact_schema_commands_test.go`.
- WP05 → WP25: `artifact_read_test.go`.
- WP09 → WP25: `artifact_replication_test.go`.
- WP12 → WP25: `artifact_search_acceptance_test.go`.
- WP14 → WP25: `extension_enablement_test.go`.
- WP19 → WP25: `extension_changes_acceptance_test.go`.
- WP21 → WP25: `extension_charter_acceptance_test.go`.

Original commits, source identities, implementation/review records and historical path lists remain unchanged. Only future write scope transfers. All eight files are test-only; production ownership, signed contracts, dependencies, task IDs and lane identities remain unchanged. The current frontmatter is authoritative for prospective ownership. The files exist in approved implementation lanes but are absent from the primary planning checkout, so WP25 create_intent allows normal integration, not duplicate creation or rewritten authorship. WP20 and WP22 retain their current own-file fixes. WP24 retains its pending actual-path default/performance gate.

## Required repairs

Use the approved OS-aware config helper/actual os.UserConfigDir, isolate and pin every fixture's XDG environment for all its child processes, and retain the actual selected path. On Darwin the path is HOME/Library/Application Support; an inherited XDG directory must never silently select another fixture's config. Actual registration must create a nonempty registry at the path used by detectors. A positive control must prove an explicit management read or deliberately activated read detector observes that same registry; default native commands must remain registry-inert. Keep all malformed/disabled/crashing/module-start/private-write/unchanged-native assertions. Never call a skipped strace test a macOS syscall observation.

For the WP03/04/05 sentinel tests, preserve arbitrary-file integrity controls and place any registry-specific trap at the actual selected registry. Do not credit arbitrary old-path sentinels as actual-registry read proof. Keep each original refusal, bound, signature, exact byte and no-effect assertion effective.

For WP09 use the selected Git directory's actual hn keyring. The independently initialized destination has its own legitimate identity: require absence of the source actor's private key and unchanged destination actor/key bytes, not absence of all destination keys. Retain the separate source-intent/private-output/public-object leak assertions. Add a deliberate detector control for the source-key condition; do not copy private source state into public transport.

No runtime implementation, authority, wire format, default setup, native dependency or published limit changes belong to this maintenance. A discovered product defect is a separate owner decision, not permission to weaken a test. Every changed test must still exercise the actual caller and fail for its asserted defect when deliberately broken.

## Adoption and qualification

Before adoption run normal finalizer validation, normal refresh/finalization and fresh analysis. Compare all 26 WP→lane identities, dependency graph, 107 subtask IDs, production ownership, frozen contract hashes and canonical lifecycle/verdict history. No overlap/collapse/renumbering or status edit is allowed. If normal tooling cannot preserve those invariants, use original-owner test-only maintenance/re-review loops instead.

WP25 then implements the bounded repairs in its computed final integrated lane, records exact before/after source/input maps, proves production/module bytes unchanged for reuse of WP24 performance evidence, runs affected standard/combined/race tests and full fixed-source qualification, and executes all eight exact Linux/macOS platform/variant binaries. Its independent reviewer must verify these corrections and detector controls. Earlier Linux results remain historical evidence only; missing Mac observations continue to block full qualification. This assignment changes no beta acceptance, main branch or release.
