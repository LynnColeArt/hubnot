# Test fixture config-path portability correction

Root decision, 2026-10-31. The full optional-extension mission remains active.

A read-only preflight and independent static audit found that root registration test fixtures set both an isolated HOME and XDG_CONFIG_HOME, but several direct-file assertions and malformed-registry controls assume that XDG_CONFIG_HOME is the effective config directory on every platform. Go os.UserConfigDir uses HOME/Library/Application Support on Darwin. The product already uses this OS-aware function correctly. No Mac execution or product configuration defect is inferred from this static finding.

WP13 owns the narrow correction in extension_registration_test.go: direct reads, mutations, strace paths and malformed-registry detectors must target the same effective isolated OS config directory as the invoked CLI. Preserve Linux and Darwin behavior, actual negative controls, isolation, all production bytes and native defaults. Explicit path unit tests under internal/extension are not defective merely because their supplied path is named config. WP16 separately corrects its owned hostFixture direct registry mutation as part of its active implementation.

Use normal owner lifecycle and a fresh independent review. Keep the full source/input identity, meaningful Linux positive/negative controls and platform scope honest; Darwin runtime proof still belongs to final exact-package qualification. The correction does not waive or alter frozen default, trust or performance contracts.

Audit evidence is external at /home/lynn/Documents/Codex/2026-10-24/wa/work/optional-extensions-config-fixture-audit/audit.md. This decision precedes the test correction; no missing Mac observation is marked passed.
